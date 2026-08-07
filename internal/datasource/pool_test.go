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
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDatasource records whether it was closed so tests can assert on lifecycle.
type fakeDatasource struct {
	id     int
	closed atomic.Bool
}

func (f *fakeDatasource) QueryNodes(context.Context, QueryConfig) ([]NodeRow, error) {
	return nil, nil
}

func (f *fakeDatasource) Close() error {
	f.closed.Store(true)
	return nil
}

// newTestPool returns a Pool whose constructor hands out fakeDatasources and counts calls,
// so tests can tell reuse from reconstruction without opening real connections.
func newTestPool() (*Pool, *atomic.Int32) {
	var built atomic.Int32
	p := &Pool{
		entries: make(map[PoolKey]*poolEntry),
		newDatasource: func(SourceType, Config) (Datasource, error) {
			return &fakeDatasource{id: int(built.Add(1))}, nil
		},
	}
	return p, &built
}

func testConfig() Config {
	return Config{
		Host:     "db.example.com",
		Port:     3306,
		Username: "lynq",
		Password: "secret",
		Database: "tenants",
	}
}

// TestPool_ReusesDatasourceAcrossSyncs is the point of the pool: a hub that syncs repeatedly
// with unchanged settings must connect once, not once per sync. Rebuilding every sync meant
// a DNS lookup, TCP handshake, TLS negotiation and auth round trip every syncInterval, which
// in production failed intermittently and made the adapter's pool settings meaningless.
func TestPool_ReusesDatasourceAcrossSyncs(t *testing.T) {
	p, built := newTestPool()
	key := PoolKey{Namespace: "default", Name: "hub"}

	// Given a hub that has connected once
	first, err := p.Acquire(key, SourceTypeMySQL, testConfig())
	require.NoError(t, err)

	// When it syncs many more times with the same settings
	for i := 0; i < 10; i++ {
		again, err := p.Acquire(key, SourceTypeMySQL, testConfig())
		require.NoError(t, err)

		// Then it gets the very same datasource back
		assert.Same(t, first, again)
	}

	assert.Equal(t, int32(1), built.Load(), "datasource should be constructed exactly once")
	assert.False(t, first.(*fakeDatasource).closed.Load(), "the live datasource must stay open")
}

// TestPool_RebuildsWhenConnectionSettingsChange covers credential rotation and endpoint
// changes: the cached connection is no longer valid for the new settings, so it must be
// closed and replaced rather than silently reused.
func TestPool_RebuildsWhenConnectionSettingsChange(t *testing.T) {
	changes := []struct {
		name   string
		mutate func(*Config)
	}{
		{"rotated password", func(c *Config) { c.Password = "rotated" }},
		{"new host", func(c *Config) { c.Host = "replica.example.com" }},
		{"new port", func(c *Config) { c.Port = 3307 }},
		{"new user", func(c *Config) { c.Username = "other" }},
		{"new database", func(c *Config) { c.Database = "other" }},
		{"resized pool", func(c *Config) { c.MaxOpenConns = 50 }},
		{"new conn lifetime", func(c *Config) { c.ConnMaxLifetime = "1m" }},
	}

	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			p, built := newTestPool()
			key := PoolKey{Namespace: "default", Name: "hub"}

			// Given a hub connected with its original settings
			original, err := p.Acquire(key, SourceTypeMySQL, testConfig())
			require.NoError(t, err)

			// When those settings change
			changed := testConfig()
			tc.mutate(&changed)
			replacement, err := p.Acquire(key, SourceTypeMySQL, changed)
			require.NoError(t, err)

			// Then a new datasource is built and the stale one is closed
			assert.NotSame(t, original, replacement)
			assert.Equal(t, int32(2), built.Load())
			assert.True(t, original.(*fakeDatasource).closed.Load(),
				"the superseded datasource must be closed, not leaked")
			assert.Equal(t, 1, p.Len(), "only one datasource per hub may be held")
		})
	}
}

// TestPool_ReleaseClosesAndForgets covers hub deletion. Nothing else will ever close this
// connection, so Release must.
func TestPool_ReleaseClosesAndForgets(t *testing.T) {
	p, _ := newTestPool()
	key := PoolKey{Namespace: "default", Name: "hub"}

	// Given a connected hub
	ds, err := p.Acquire(key, SourceTypeMySQL, testConfig())
	require.NoError(t, err)
	require.Equal(t, 1, p.Len())

	// When the hub goes away
	p.Release(key)

	// Then its connection is closed and dropped
	assert.True(t, ds.(*fakeDatasource).closed.Load())
	assert.Equal(t, 0, p.Len())

	// And releasing again is harmless — callers release unconditionally on delete paths
	assert.NotPanics(t, func() { p.Release(key) })
	assert.NotPanics(t, func() { p.Release(PoolKey{Namespace: "default", Name: "never-seen"}) })
}

// TestPool_KeysAreIsolatedPerHub guards against one hub's datasource being handed to
// another that happens to share connection settings.
func TestPool_KeysAreIsolatedPerHub(t *testing.T) {
	p, built := newTestPool()
	a := PoolKey{Namespace: "team-a", Name: "hub"}
	b := PoolKey{Namespace: "team-b", Name: "hub"}

	// Given two hubs with identical connection settings
	dsA, err := p.Acquire(a, SourceTypeMySQL, testConfig())
	require.NoError(t, err)
	dsB, err := p.Acquire(b, SourceTypeMySQL, testConfig())
	require.NoError(t, err)

	// Then each holds its own datasource
	assert.NotSame(t, dsA, dsB)
	assert.Equal(t, int32(2), built.Load())

	// And releasing one leaves the other usable
	p.Release(a)
	assert.True(t, dsA.(*fakeDatasource).closed.Load())
	assert.False(t, dsB.(*fakeDatasource).closed.Load())
	assert.Equal(t, 1, p.Len())
}

// TestPool_ConstructionFailureIsNotCached ensures a failed connection attempt does not
// poison the entry. Connection failures are transient (DNS blips, deadline exceeded), so
// the next sync must be free to retry.
func TestPool_ConstructionFailureIsNotCached(t *testing.T) {
	var attempts atomic.Int32
	wantErr := errors.New("failed to ping MySQL: i/o timeout")
	p := &Pool{
		entries: make(map[PoolKey]*poolEntry),
		newDatasource: func(SourceType, Config) (Datasource, error) {
			if attempts.Add(1) == 1 {
				return nil, wantErr
			}
			return &fakeDatasource{}, nil
		},
	}
	key := PoolKey{Namespace: "default", Name: "hub"}

	// Given a sync whose connection attempt fails
	ds, err := p.Acquire(key, SourceTypeMySQL, testConfig())
	require.ErrorIs(t, err, wantErr)
	assert.Nil(t, ds)
	assert.Equal(t, 0, p.Len(), "a failed attempt must not be cached")

	// When the next sync retries
	ds, err = p.Acquire(key, SourceTypeMySQL, testConfig())

	// Then it succeeds and is cached
	require.NoError(t, err)
	assert.NotNil(t, ds)
	assert.Equal(t, 1, p.Len())
}

// TestPool_ReplacementFailureLeavesNoStaleEntry checks the failure path of a settings
// change: the old datasource is already closed, so keeping it cached would hand callers a
// closed connection forever.
func TestPool_ReplacementFailureLeavesNoStaleEntry(t *testing.T) {
	var attempts atomic.Int32
	p := &Pool{
		entries: make(map[PoolKey]*poolEntry),
		newDatasource: func(SourceType, Config) (Datasource, error) {
			if attempts.Add(1) == 1 {
				return &fakeDatasource{}, nil
			}
			return nil, errors.New("failed to ping MySQL: i/o timeout")
		},
	}
	key := PoolKey{Namespace: "default", Name: "hub"}

	original, err := p.Acquire(key, SourceTypeMySQL, testConfig())
	require.NoError(t, err)

	changed := testConfig()
	changed.Password = "rotated"
	_, err = p.Acquire(key, SourceTypeMySQL, changed)

	require.Error(t, err)
	assert.True(t, original.(*fakeDatasource).closed.Load(), "the stale datasource is closed")
	assert.Equal(t, 0, p.Len(), "no closed datasource may remain cached")
}

// TestPool_CloseAllReleasesEverything covers shutdown, which the process performs after
// mgr.Start returns rather than from a Runnable (see the note in pool.go).
func TestPool_CloseAllReleasesEverything(t *testing.T) {
	p, _ := newTestPool()

	names := []string{"hub-a", "hub-b", "hub-c"}
	held := make([]Datasource, 0, len(names))
	for _, name := range names {
		ds, err := p.Acquire(PoolKey{Namespace: "default", Name: name}, SourceTypeMySQL, testConfig())
		require.NoError(t, err)
		held = append(held, ds)
	}
	require.Equal(t, 3, p.Len())

	// When the process shuts down
	p.CloseAll()

	// Then every connection is closed
	for i, ds := range held {
		assert.True(t, ds.(*fakeDatasource).closed.Load(), "datasource %d should be closed", i)
	}
	assert.Equal(t, 0, p.Len())
}

// TestPool_ConstructionDoesNotBlockOtherHubs pins the reason construction happens outside the
// pool mutex: building a datasource performs a network round trip with a multi-second timeout,
// so holding the lock across it would serialize every hub behind one unreachable database.
func TestPool_ConstructionDoesNotBlockOtherHubs(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	p := &Pool{
		entries: make(map[PoolKey]*poolEntry),
		newDatasource: func(_ SourceType, config Config) (Datasource, error) {
			if config.Host == "slow.example.com" {
				entered <- struct{}{}
				<-release // Stand in for a connect attempt hanging until its timeout
			}
			return &fakeDatasource{}, nil
		},
	}

	// Given one hub stuck constructing a connection to an unreachable database
	go func() {
		slow := testConfig()
		slow.Host = "slow.example.com"
		_, _ = p.Acquire(PoolKey{Namespace: "default", Name: "slow-hub"}, SourceTypeMySQL, slow)
	}()
	<-entered

	// When an unrelated hub acquires its own connection
	done := make(chan error, 1)
	go func() {
		_, err := p.Acquire(PoolKey{Namespace: "default", Name: "other-hub"}, SourceTypeMySQL, testConfig())
		done <- err
	}()

	// Then it is not blocked behind the stuck one
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("Acquire for an unrelated hub blocked behind another hub's construction")
	}
	close(release)
}

// TestConnectionFingerprint_CoversEveryConfigField fails when a field is added to Config
// without being added to connectionFingerprint.
//
// That omission would otherwise be silent and permanent: the new setting would change, the
// fingerprint would not, and Acquire would keep handing out a datasource built for the old
// settings forever. A reflection sweep turns that into a test failure at the moment the field
// is introduced.
func TestConnectionFingerprint_CoversEveryConfigField(t *testing.T) {
	base := testConfig()
	baseline := connectionFingerprint(SourceTypeMySQL, base)

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		t.Run(field.Name, func(t *testing.T) {
			mutated := base
			v := reflect.ValueOf(&mutated).Elem().Field(i)

			// Perturb the field to a value that differs from the baseline.
			switch v.Kind() {
			case reflect.String:
				v.SetString(v.String() + "-changed")
			case reflect.Int, reflect.Int32, reflect.Int64:
				v.SetInt(v.Int() + 1)
			default:
				t.Fatalf("Config.%s has unhandled kind %s — extend this test and "+
					"connectionFingerprint together", field.Name, v.Kind())
			}

			assert.NotEqual(t, baseline, connectionFingerprint(SourceTypeMySQL, mutated),
				"changing Config.%s did not change the fingerprint, so a hub that changes this "+
					"setting would silently keep using its old connection. Add the field to "+
					"connectionFingerprint.", field.Name)
		})
	}
}

// TestPool_ConcurrentAcquireIsSafe exercises the lock: hub-concurrency defaults to 3, so
// distinct hubs reconcile in parallel against the shared pool. Run with -race.
func TestPool_ConcurrentAcquireIsSafe(t *testing.T) {
	p, _ := newTestPool()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		for _, name := range []string{"hub-a", "hub-b", "hub-c"} {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				_, err := p.Acquire(PoolKey{Namespace: "default", Name: name}, SourceTypeMySQL, testConfig())
				assert.NoError(t, err)
			}(name)
		}
	}
	wg.Wait()

	assert.Equal(t, 3, p.Len(), "one datasource per distinct hub")
}

// TestConnectionFingerprint_IsUnambiguous guards the length-prefixed hashing: without it,
// shifting characters between adjacent fields would collide and a genuine settings change
// could be mistaken for a no-op.
func TestConnectionFingerprint_IsUnambiguous(t *testing.T) {
	a := Config{Host: "a", Database: "bc"}
	b := Config{Host: "ab", Database: "c"}

	assert.NotEqual(t,
		connectionFingerprint(SourceTypeMySQL, a),
		connectionFingerprint(SourceTypeMySQL, b))

	// Same settings must hash identically, or the pool would rebuild on every sync
	assert.Equal(t,
		connectionFingerprint(SourceTypeMySQL, testConfig()),
		connectionFingerprint(SourceTypeMySQL, testConfig()))

	// Source type is part of the identity
	assert.NotEqual(t,
		connectionFingerprint(SourceTypeMySQL, testConfig()),
		connectionFingerprint(SourceTypePostgreSQL, testConfig()))
}
