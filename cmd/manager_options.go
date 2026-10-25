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
	"flag"
	"fmt"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	ctrl "sigs.k8s.io/controller-runtime"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// kubeAPIWarningsDeduplicate logs each distinct API server warning once per process.
	kubeAPIWarningsDeduplicate = "deduplicate"
	// kubeAPIWarningsAll logs every API server warning, as controller-runtime does by default.
	kubeAPIWarningsAll = "all"
)

// leaderElectionTuning holds the leader election timings. The flag defaults are
// controller-runtime's own defaults, so leaving every flag unset changes nothing.
type leaderElectionTuning struct {
	LeaseDuration   time.Duration
	RenewDeadline   time.Duration
	RetryPeriod     time.Duration
	ReleaseOnCancel bool
}

func bindLeaderElectionFlags(fs *flag.FlagSet) *leaderElectionTuning {
	t := &leaderElectionTuning{}
	fs.DurationVar(&t.LeaseDuration, "leader-elect-lease-duration", 15*time.Second,
		"How long non-leader candidates wait after the last observed renewal before trying to take the lease. "+
			"Must be greater than --leader-elect-renew-deadline.")
	fs.DurationVar(&t.RenewDeadline, "leader-elect-renew-deadline", 10*time.Second,
		"How long the leader keeps retrying to renew the lease before it gives up leadership and exits. "+
			"Also the HTTP timeout of each lease request. Must be greater than 1.2x --leader-elect-retry-period.")
	fs.DurationVar(&t.RetryPeriod, "leader-elect-retry-period", 2*time.Second,
		"How long candidates wait between attempts to acquire or renew the lease.")
	fs.BoolVar(&t.ReleaseOnCancel, "leader-elect-release-on-cancel", false,
		"Release the lease on a graceful shutdown so a standby replica takes over without waiting "+
			"--leader-elect-lease-duration. The lease is released only after the controllers have stopped.")
	return t
}

// validate rejects timings client-go's leader elector would reject anyway, but at startup
// with the flag names instead of from inside mgr.Start.
func (t *leaderElectionTuning) validate() error {
	if t.LeaseDuration <= 0 || t.RenewDeadline <= 0 || t.RetryPeriod <= 0 {
		return fmt.Errorf("leader election durations must be positive: lease-duration=%s renew-deadline=%s retry-period=%s",
			t.LeaseDuration, t.RenewDeadline, t.RetryPeriod)
	}
	if t.LeaseDuration <= t.RenewDeadline {
		return fmt.Errorf("--leader-elect-lease-duration (%s) must be greater than --leader-elect-renew-deadline (%s)",
			t.LeaseDuration, t.RenewDeadline)
	}
	if t.RenewDeadline <= time.Duration(leaderelection.JitterFactor*float64(t.RetryPeriod)) {
		return fmt.Errorf("--leader-elect-renew-deadline (%s) must be greater than %.1fx --leader-elect-retry-period (%s)",
			t.RenewDeadline, leaderelection.JitterFactor, t.RetryPeriod)
	}
	return nil
}

func (t *leaderElectionTuning) applyTo(opts *ctrl.Options) {
	leaseDuration, renewDeadline, retryPeriod := t.LeaseDuration, t.RenewDeadline, t.RetryPeriod
	opts.LeaseDuration = &leaseDuration
	opts.RenewDeadline = &renewDeadline
	opts.RetryPeriod = &retryPeriod
	opts.LeaderElectionReleaseOnCancel = t.ReleaseOnCancel
}

// newKubeAPIWarningHandler builds the handler for warnings the API server returns in response
// headers (deprecated API versions, unknown fields, ...). Every client the manager builds copies
// the rest.Config, handler included, so one handler instance covers the manager client, the
// APIReader, the informers, the event recorder and leader election, and deduplication is
// process-wide. A deprecated version used by a LynqForm otherwise logs once per apply of every
// node, which drowned out every other log line.
func newKubeAPIWarningHandler(mode string) (rest.WarningHandlerWithContext, error) {
	switch mode {
	case kubeAPIWarningsDeduplicate:
		return crlog.NewKubeAPIWarningLogger(crlog.KubeAPIWarningLoggerOptions{Deduplicate: true}), nil
	case kubeAPIWarningsAll:
		return crlog.NewKubeAPIWarningLogger(crlog.KubeAPIWarningLoggerOptions{Deduplicate: false}), nil
	default:
		return nil, fmt.Errorf("--kube-api-warnings must be %q or %q, got %q",
			kubeAPIWarningsDeduplicate, kubeAPIWarningsAll, mode)
	}
}

// withKubeAPIWarningHandler returns a copy of cfg that reports API server warnings through h.
func withKubeAPIWarningHandler(cfg *rest.Config, h rest.WarningHandlerWithContext) *rest.Config {
	cfg = rest.CopyConfig(cfg)
	cfg.WarningHandler = nil
	cfg.WarningHandlerWithContext = h
	return cfg
}
