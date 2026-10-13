//go:build unit

package leader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestKubeDefaults(t *testing.T) {
	nsFile := filepath.Join(t.TempDir(), "namespace")
	require.NoError(t, os.WriteFile(nsFile, []byte("networking\n"), 0o600))
	t.Cleanup(func(old string) func() { return func() { namespaceFile = old } }(namespaceFile))
	namespaceFile = nsFile
	t.Setenv("POD_NAME", "ddup-abc")
	t.Setenv("POD_IP", "10.244.0.18")

	client := fake.NewSimpleClientset()
	e, err := NewKube(KubeOpts{
		Name: "ddup", Client: client,
		LeaseDuration: 1500 * time.Millisecond, RenewDeadline: time.Second, RetryPeriod: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	go e.Run(t.Context())
	require.Eventually(t, e.IsLeader, 3*time.Second, 50*time.Millisecond)

	lease, err := client.CoordinationV1().Leases("networking").Get(t.Context(), "ddup", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "10.244.0.18", *lease.Spec.HolderIdentity, "POD_IP wins over POD_NAME")
	assert.Equal(t, "10.244.0.18", e.Leader())
}

func TestKubeNoNamespace(t *testing.T) {
	t.Cleanup(func(old string) func() { return func() { namespaceFile = old } }(namespaceFile))
	namespaceFile = filepath.Join(t.TempDir(), "missing")
	_, err := NewKube(KubeOpts{Name: "ddup", Client: fake.NewSimpleClientset()})
	require.ErrorContains(t, err, "pod namespace")
}
