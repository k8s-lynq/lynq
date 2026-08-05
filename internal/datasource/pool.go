/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datasource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// PoolKey identifies the owner of a pooled datasource — one entry per LynqHub.
type PoolKey struct {
	Namespace string
	Name      string
}

func (k PoolKey) String() string { return k.Namespace + "/" + k.Name }

// Pool keeps one live Datasource per LynqHub across reconciles.
//
// Without it, every sync built a fresh datasource and closed it immediately, so the
// connection-pool settings an adapter configures (MaxOpenConns, MaxIdleConns,
// ConnMaxLifetime) never applied to more than a single sync. At a 30s syncInterval that is
// ~2,880 connection setups per hub per day — each one a DNS lookup, TCP handshake, TLS
// negotiation and auth round trip, and each one an opportunity to fail. In production this
// surfaced as intermittent `failed to ping MySQL: i/o timeout` / `context deadline exceeded`
// errors on roughly 6% of syncs, purely from transient DNS and connect failures.
//
// Reusing the datasource means a sync normally issues a query on an already-established
// connection. database/sql transparently replaces connections it finds broken, and
// ConnMaxLifetime still recycles them periodically, so endpoint changes (e.g. an RDS
// failover) are picked up without a restart.
//
// A cached entry is discarded and rebuilt when the hub's connection settings change —
// including a rotated password, since the credential is part of the fingerprint.
type Pool struct {
	mu      sync.Mutex
	entries map[PoolKey]*poolEntry

	// newDatasource is the constructor used to build entries. It exists so tests can
	// substitute a fake without opening real connections; production uses NewDatasource.
	newDatasource func(SourceType, Config) (Datasource, error)
}

type poolEntry struct {
	datasource  Datasource
	fingerprint string
}

// NewPool creates an empty Pool backed by the real datasource constructor.
func NewPool() *Pool {
	return &Pool{
		entries:       make(map[PoolKey]*poolEntry),
		newDatasource: NewDatasource,
	}
}

// Acquire returns the datasource for key, creating it if absent and replacing it if the
// connection settings have changed since it was created.
//
// The returned Datasource is owned by the Pool. Callers MUST NOT Close it — doing so would
// break every later sync for that hub. Use Release when the hub goes away.
//
// A construction failure caches nothing, so the next sync retries with a fresh attempt.
func (p *Pool) Acquire(key PoolKey, sourceType SourceType, config Config) (Datasource, error) {
	fingerprint := connectionFingerprint(sourceType, config)

	p.mu.Lock()
	defer p.mu.Unlock()

	if entry, ok := p.entries[key]; ok {
		if entry.fingerprint == fingerprint {
			return entry.datasource, nil
		}
		// Connection settings changed (host, credentials, pool sizing, ...). Drop the stale
		// datasource before building its replacement so we never hold two pools for one hub.
		_ = entry.datasource.Close() // Best effort close
		delete(p.entries, key)
	}

	ds, err := p.newDatasource(sourceType, config)
	if err != nil {
		return nil, err
	}

	p.entries[key] = &poolEntry{datasource: ds, fingerprint: fingerprint}
	return ds, nil
}

// Release closes and forgets the datasource for key. Safe to call for a key that was never
// acquired, so callers can invoke it unconditionally on any path where the hub is gone.
func (p *Pool) Release(key PoolKey) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry, ok := p.entries[key]
	if !ok {
		return
	}
	_ = entry.datasource.Close() // Best effort close
	delete(p.entries, key)
}

// Len reports how many datasources are currently held.
func (p *Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}

// CloseAll closes every held datasource and empties the pool.
func (p *Pool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, entry := range p.entries {
		_ = entry.datasource.Close() // Best effort close
		delete(p.entries, key)
	}
}

// Start implements manager.Runnable so the Pool's lifetime is tied to the manager's: it
// holds connections open until shutdown, then closes them all.
func (p *Pool) Start(ctx context.Context) error {
	<-ctx.Done()
	p.CloseAll()
	return nil
}

// connectionFingerprint hashes every field that determines which server we connect to, as
// whom, and how the underlying pool is sized. A change in any of them must produce a new
// datasource rather than reuse of the old one.
//
// The value is hashed rather than kept verbatim because Config carries the database
// password, and the fingerprint is held in memory for the process's lifetime.
func connectionFingerprint(sourceType SourceType, config Config) string {
	h := sha256.New()
	// Length-prefix each field so that adjacent values cannot be shifted between fields to
	// produce the same digest (e.g. host "a" + db "bc" vs host "ab" + db "c").
	for _, field := range []string{
		string(sourceType),
		config.Host,
		fmt.Sprint(config.Port),
		config.Username,
		config.Password,
		config.Database,
		fmt.Sprint(config.MaxOpenConns),
		fmt.Sprint(config.MaxIdleConns),
		config.ConnMaxLifetime,
	} {
		// hash.Hash.Write never returns an error, so Fprintf cannot fail here.
		_, _ = fmt.Fprintf(h, "%d:%s|", len(field), field)
	}
	return hex.EncodeToString(h.Sum(nil))
}
