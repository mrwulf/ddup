package leader

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// Where Kubernetes mounts the namespace of the pod; overridden in tests
var namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// KubeOpts configures the Kubernetes Lease based elector
type KubeOpts struct {
	// Name of the Lease object
	Name string
	// Namespace of the Lease object; defaults to the namespace the pod runs in
	Namespace string
	// Identity of this instance and must be unique per replica; defaults to $POD_IP, then $POD_NAME, then the hostname
	// Standby instances forward API requests to the leader, which only works when the identity is the IP address of the pod
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
	var err error
	if opts.Identity == "" {
		opts.Identity = os.Getenv("POD_IP")
	}
	if opts.Identity == "" {
		opts.Identity = os.Getenv("POD_NAME")
	}
	if opts.Identity == "" {
		opts.Identity, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to determine the instance identity: %w", err)
		}
	}
	if opts.Namespace == "" {
		b, err := os.ReadFile(namespaceFile)
		if err != nil {
			return nil, fmt.Errorf("leaderElection.namespace is not set and the pod namespace can't be read: %w", err)
		}
		opts.Namespace = strings.TrimSpace(string(b))
	}

	client := opts.Client
	if client == nil {
		var rc *rest.Config
		rc, err = rest.InClusterConfig()
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

func (k *kubeElector) Leader() string { return k.le.GetLeader() }
