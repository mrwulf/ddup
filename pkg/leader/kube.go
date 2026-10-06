package leader

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// KubeOpts configures the Kubernetes Lease based elector
type KubeOpts struct {
	// Namespace and Name of the Lease object
	Namespace string
	Name      string
	// Identity of this instance; must be unique per replica (e.g. the pod name)
	Identity      string
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
	// Client is optional; defaults to the in-cluster config
	Client kubernetes.Interface
}

type kubeElector struct {
	le       *leaderelection.LeaderElector
	isLeader atomic.Bool
}

// NewKube returns an Elector backed by a coordination.k8s.io Lease, using client-go's leaderelection
func NewKube(opts KubeOpts) (Elector, error) {
	client := opts.Client
	if client == nil {
		rc, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("leader election needs to run in a cluster: %w", err)
		}
		client, err = kubernetes.NewForConfig(rc)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
		}
	}

	k := &kubeElector{}
	le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta:  metav1.ObjectMeta{Namespace: opts.Namespace, Name: opts.Name},
			Client:     client.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{Identity: opts.Identity},
		},
		LeaseDuration: opts.LeaseDuration,
		RenewDeadline: opts.RenewDeadline,
		RetryPeriod:   opts.RetryPeriod,
		// Hand the lease back on shutdown so a rolling update fails over immediately
		ReleaseOnCancel: true,
		Name:            opts.Name,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(context.Context) {
				k.isLeader.Store(true)
				slog.Info("Acquired leadership", "identity", opts.Identity)
			},
			OnStoppedLeading: func() {
				k.isLeader.Store(false)
				slog.Info("Lost leadership", "identity", opts.Identity)
			},
			OnNewLeader: func(id string) {
				if id != opts.Identity {
					slog.Info("Current leader", "identity", id)
				}
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("invalid leader election configuration: %w", err)
	}
	k.le = le
	return k, nil
}

func (k *kubeElector) Run(ctx context.Context) error {
	// le.Run returns when leadership is lost; keep campaigning until we're shut down
	for ctx.Err() == nil {
		k.le.Run(ctx)
	}
	return nil
}

func (k *kubeElector) IsLeader() bool { return k.isLeader.Load() }
