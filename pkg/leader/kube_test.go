//go:build unit

package leader

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestElector(t *testing.T, client *fake.Clientset, id string) Elector {
	t.Helper()
	e, err := NewKube(KubeOpts{
		Namespace: "default", Name: "ddup", Identity: id, Client: client,
		LeaseDuration: 1500 * time.Millisecond, RenewDeadline: time.Second, RetryPeriod: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	return e
}

func TestKubeFailover(t *testing.T) {
	client := fake.NewSimpleClientset()
	a, b := newTestElector(t, client, "a"), newTestElector(t, client, "b")

	ctxA, cancelA := context.WithCancel(t.Context())
	go a.Run(ctxA)
	require.Eventually(t, a.IsLeader, 3*time.Second, 50*time.Millisecond)

	go b.Run(t.Context())
	time.Sleep(500 * time.Millisecond)
	assert.False(t, b.IsLeader(), "only one leader at a time")

	// a shuts down and releases the lease, b takes over
	cancelA()
	require.Eventually(t, b.IsLeader, 5*time.Second, 50*time.Millisecond)
	assert.False(t, a.IsLeader())
}
