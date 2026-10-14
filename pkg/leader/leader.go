// Package leader decides which ddup replica may act (write DNS, send webhooks)
// All replicas keep running health checks, so a standby has warm state when it takes over
package leader

import "context"

// Elector reports whether this instance currently holds leadership
type Elector interface {
	// Run campaigns for leadership until ctx is canceled, re-campaigning after leadership is lost
	// It blocks, so it's meant to run as a service
	Run(ctx context.Context) error
	// IsLeader returns true while this instance holds leadership
	// It's cheap and safe to call from any goroutine
	IsLeader() bool
	// Leader returns the identity of the current leader, or an empty string if it's unknown or there's no election
	Leader() string
}

// Always is the Elector used when leader election is disabled: a single instance is always the leader
type Always struct{}

func (Always) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (Always) IsLeader() bool { return true }

func (Always) Leader() string { return "" }
