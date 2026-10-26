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

package main

import (
	"context"
	"flag"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

const deprecationWarning = "Version v1beta2 of the KafkaTopic API is deprecated. Please use the v1 version instead."

// logRecorder is a logger that keeps every message it was given.
type logRecorder struct {
	mu       sync.Mutex
	messages []string
}

func (r *logRecorder) logger() logr.Logger {
	return funcr.New(func(_, args string) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.messages = append(r.messages, args)
	}, funcr.Options{})
}

func (r *logRecorder) count(substr string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, m := range r.messages {
		if strings.Contains(m, substr) {
			n++
		}
	}
	return n
}

func parseLeaderElectionFlags(t *testing.T, args ...string) *leaderElectionTuning {
	t.Helper()
	fs := flag.NewFlagSet("manager", flag.ContinueOnError)
	tuning := bindLeaderElectionFlags(fs)
	require.NoError(t, fs.Parse(args))
	return tuning
}

func TestLeaderElectionFlags_DefaultsMatchControllerRuntime(t *testing.T) {
	// Given no leader election tuning flags
	tuning := parseLeaderElectionFlags(t)

	// When the manager options are built
	opts := ctrl.Options{}
	tuning.applyTo(&opts)

	// Then the timings are controller-runtime's defaults and the lease is not released on shutdown
	require.NoError(t, tuning.validate())
	assert.Equal(t, 15*time.Second, *opts.LeaseDuration)
	assert.Equal(t, 10*time.Second, *opts.RenewDeadline)
	assert.Equal(t, 2*time.Second, *opts.RetryPeriod)
	assert.False(t, opts.LeaderElectionReleaseOnCancel)
}

func TestLeaderElectionFlags_TunedValuesReachManagerOptions(t *testing.T) {
	// Given the operator tunes every leader election flag
	tuning := parseLeaderElectionFlags(t,
		"--leader-elect-lease-duration=30s",
		"--leader-elect-renew-deadline=20s",
		"--leader-elect-retry-period=5s",
		"--leader-elect-release-on-cancel",
	)

	// When the manager options are built
	opts := ctrl.Options{}
	tuning.applyTo(&opts)

	// Then the manager receives exactly those values
	require.NoError(t, tuning.validate())
	assert.Equal(t, 30*time.Second, *opts.LeaseDuration)
	assert.Equal(t, 20*time.Second, *opts.RenewDeadline)
	assert.Equal(t, 5*time.Second, *opts.RetryPeriod)
	assert.True(t, opts.LeaderElectionReleaseOnCancel)
}

func TestLeaderElectionFlags_RejectsTimingsTheLeaderElectorWouldRefuse(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "lease duration not longer than renew deadline",
			args:    []string{"--leader-elect-lease-duration=10s", "--leader-elect-renew-deadline=10s"},
			wantErr: "--leader-elect-lease-duration",
		},
		{
			name:    "renew deadline within jittered retry period",
			args:    []string{"--leader-elect-renew-deadline=6s", "--leader-elect-retry-period=5s"},
			wantErr: "--leader-elect-renew-deadline",
		},
		{
			name:    "non-positive retry period",
			args:    []string{"--leader-elect-retry-period=0s"},
			wantErr: "must be positive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tuning := parseLeaderElectionFlags(t, tt.args...)
			err := tuning.validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestKubeAPIWarningHandler_LogsRepeatedWarningOnceWhenDeduplicating(t *testing.T) {
	tests := []struct {
		name          string
		mode          string
		wantRepeated  int
		wantDifferent int
	}{
		{name: "deduplicate logs each distinct warning once", mode: kubeAPIWarningsDeduplicate, wantRepeated: 1, wantDifferent: 1},
		{name: "all logs every occurrence", mode: kubeAPIWarningsAll, wantRepeated: 3, wantDifferent: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a warning handler and a reconcile logger
			h, err := newKubeAPIWarningHandler(tt.mode)
			require.NoError(t, err)
			rec := &logRecorder{}
			ctx := crlog.IntoContext(context.Background(), rec.logger())

			// When the API server returns the same deprecation warning three times, then another warning
			for range 3 {
				h.HandleWarningHeaderWithContext(ctx, 299, "-", deprecationWarning)
			}
			h.HandleWarningHeaderWithContext(ctx, 299, "-", "unknown field \"spec.foo\"")

			// Then the repeated warning is logged per the mode and a new warning is still logged
			assert.Equal(t, tt.wantRepeated, rec.count("KafkaTopic API is deprecated"))
			assert.Equal(t, tt.wantDifferent, rec.count("unknown field"))
		})
	}
}

func TestKubeAPIWarningHandler_RejectsUnknownMode(t *testing.T) {
	_, err := newKubeAPIWarningHandler("none")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--kube-api-warnings")
}

func TestKubeAPIWarningHandler_SharedAcrossClientsBuiltFromTheConfig(t *testing.T) {
	// Given an API server that attaches a deprecation warning to every response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Warning", `299 - "`+deprecationWarning+`"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm","namespace":"default"}}`))
	}))
	defer server.Close()

	h, err := newKubeAPIWarningHandler(kubeAPIWarningsDeduplicate)
	require.NoError(t, err)
	cfg := withKubeAPIWarningHandler(&rest.Config{Host: server.URL}, h)

	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
	newClient := func() client.Client {
		c, err := client.New(cfg, client.Options{Mapper: mapper})
		require.NoError(t, err)
		return c
	}
	// Two independent clients stand in for the manager client and the APIReader
	managerClient, apiReader := newClient(), newClient()

	rec := &logRecorder{}
	ctx := crlog.IntoContext(context.Background(), rec.logger())
	key := types.NamespacedName{Namespace: "default", Name: "cm"}

	// When both clients read repeatedly
	for range 3 {
		require.NoError(t, managerClient.Get(ctx, key, &corev1.ConfigMap{}))
		require.NoError(t, apiReader.Get(ctx, key, &corev1.ConfigMap{}))
	}

	// Then the warning is logged once for the whole process, not once per request or per client
	assert.Equal(t, 1, rec.count("KafkaTopic API is deprecated"))
}
