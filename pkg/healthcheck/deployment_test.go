package healthcheck

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	configkit "github.com/italypaleale/go-kit/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/italypaleale/ddup/pkg/config"
	"github.com/italypaleale/ddup/pkg/dns"
	"github.com/italypaleale/ddup/pkg/iplookup"
	"github.com/italypaleale/ddup/pkg/leader"
	"github.com/italypaleale/ddup/pkg/notify"
)

// These tests are the regression test of the deployed setup: pkg/config/testdata/deployment-like.yaml has the structure of the real configuration,
// and it goes through the same steps as cmd/ddup/main.go: load, validate, build the notifier and the health checker, and run
// Only the outside is replaced: the DNS provider (an in-memory zone), the health check endpoints and lookup services (local servers), ntfy and the mail relay (local servers) and the Kubernetes API (a fake client)

//nolint:gosec // Fake values for a test
const (
	deploymentConfig = "../config/testdata/deployment-like.yaml"

	// Where the file has its secrets, replaced with a folder with fake values
	secretsPrefix = "/etc/ddup/secrets"

	ntfyToken = "fake-ntfy-token"
	mailTo    = "ops@example.com"
	ddnsName  = "home.example.net"

	relayAddr = "smtp-relay.system.svc.cluster.local:2525"
)

// deploymentSecrets are the files of the secret mounted in the pod
//
//nolint:gosec // Fake values for a test
var deploymentSecrets = map[string]string{
	"CLOUDFLARE_API_TOKEN":      "fake-cloudflare-token",
	"CLOUDFLARE_ZONE_ID":        "fake-zone-id",
	"DDNS_CLOUDFLARE_API_TOKEN": "fake-ddns-token",
	"DDNS_CLOUDFLARE_ZONE_ID":   "fake-ddns-zone-id",
	"DDNS_RECORD_NAME":          ddnsName,
	"NTFY_TOKEN":                ntfyToken,
	"EMAIL_CONNECTION_STRING":   "smtp://" + relayAddr + "?fromAddress=ddup@example.com&fromName=ddup&tls=none",
	"EMAIL_TO":                  mailTo,
}

// loadDeploymentConfig loads the file into the global configuration like main does, and returns it
// The secrets are files with a trailing newline, like the ones Kubernetes mounts
func loadDeploymentConfig(t *testing.T) *config.Config {
	t.Helper()

	secretsDir := t.TempDir()
	for name, value := range deploymentSecrets {
		require.NoError(t, os.WriteFile(filepath.Join(secretsDir, name), []byte(value+"\n"), 0o600))
	}

	src, err := os.ReadFile(deploymentConfig)
	require.NoError(t, err)
	require.Contains(t, string(src), secretsPrefix)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	rendered := strings.ReplaceAll(string(src), secretsPrefix, secretsDir)
	require.NoError(t, os.WriteFile(cfgPath, []byte(rendered), 0o600)) //nolint:gosec // The path is in the temporary folder of the test
	t.Setenv("DDUP_CONFIG", cfgPath)

	cfg := config.Get()
	prev := *cfg
	t.Cleanup(func() { *cfg = prev })

	err = configkit.LoadConfig(cfg, configkit.LoadConfigOpts{
		EnvVar:  "DDUP_CONFIG",
		DirName: "ddup",
	})
	require.NoError(t, err)
	require.NoError(t, cfg.Validate(slog.Default()))
	require.NoError(t, dns.ValidateConfig(cfg))
	return cfg
}

func TestDeploymentLikeConfig_Structure(t *testing.T) {
	cfg := loadDeploymentConfig(t)

	assert.Equal(t, 30*time.Second, cfg.Interval)
	assert.True(t, cfg.Server.Enabled)
	assert.Equal(t, "0.0.0.0", cfg.Server.Bind)
	assert.Equal(t, 7401, cfg.Server.Port)

	// Two Cloudflare providers, with the credentials read from files
	require.Len(t, cfg.Providers, 2)
	require.NotNil(t, cfg.Providers["cloudflare"].Cloudflare)
	assert.Equal(t, "fake-cloudflare-token", cfg.Providers["cloudflare"].Cloudflare.APIToken)
	assert.Equal(t, "fake-zone-id", cfg.Providers["cloudflare"].Cloudflare.ZoneID)
	require.NotNil(t, cfg.Providers["cloudflare-ddns"].Cloudflare)
	assert.Equal(t, "fake-ddns-token", cfg.Providers["cloudflare-ddns"].Cloudflare.APIToken)
	assert.Equal(t, "fake-ddns-zone-id", cfg.Providers["cloudflare-ddns"].Cloudflare.ZoneID)

	require.Len(t, cfg.Domains, 3)
	byName := map[string]config.ConfigDomain{}
	for _, d := range cfg.Domains {
		byName[d.RecordName] = d
	}

	// Two VPS IPs at priority 0, and the proxied tunnel CNAME at priority 1
	ingress := byName["ingress-lb.example.com"]
	assert.Equal(t, "cloudflare", ingress.Provider)
	assert.Equal(t, 60, ingress.TTL)
	assert.Equal(t, 5*time.Second, ingress.HealthChecks.Timeout)
	assert.Equal(t, 3, ingress.HealthChecks.Attempts)
	assert.Equal(t, 3, ingress.HealthChecks.RecoverAfter)
	assert.Equal(t, "HEAD", ingress.HealthChecks.Method)
	assert.Equal(t, "418", ingress.HealthChecks.ExpectStatus)
	require.Len(t, ingress.Endpoints, 3)
	assert.Equal(t, "192.0.2.10", ingress.Endpoints[0].IP)
	assert.Equal(t, 0, ingress.Endpoints[0].Priority)
	assert.False(t, ingress.Endpoints[0].Proxied())
	assert.Equal(t, "192.0.2.11", ingress.Endpoints[1].IP)
	assert.Equal(t, 0, ingress.Endpoints[1].Priority)
	assert.Equal(t, "0a1b2c3d.cfargotunnel.com", ingress.Endpoints[2].CNAME)
	assert.Equal(t, 1, ingress.Endpoints[2].Priority)
	assert.True(t, ingress.Endpoints[2].Proxied())

	// The same endpoints with the tiers swapped, through YAML merge keys
	fast := byName["fast-lb.example.com"]
	require.Len(t, fast.Endpoints, 3)
	assert.Equal(t, "0a1b2c3d.cfargotunnel.com", fast.Endpoints[0].CNAME)
	assert.Equal(t, 0, fast.Endpoints[0].Priority)
	assert.True(t, fast.Endpoints[0].Proxied())
	assert.Equal(t, "192.0.2.10", fast.Endpoints[1].IP)
	assert.Equal(t, 1, fast.Endpoints[1].Priority)
	assert.Equal(t, "192.0.2.11", fast.Endpoints[2].IP)
	assert.Equal(t, 1, fast.Endpoints[2].Priority)
	assert.Equal(t, ingress.HealthChecks, fast.HealthChecks, "the health checks are shared with an anchor")

	// The dynamic DNS record: the name comes from a file, it follows an IPv4 address, has no URL, and lists its services
	home := byName[ddnsName]
	assert.Equal(t, "cloudflare-ddns", home.Provider)
	assert.Equal(t, 1, home.TTL)
	require.Len(t, home.Endpoints, 1)
	ep := home.Endpoints[0]
	assert.Empty(t, ep.URL)
	assert.True(t, ep.Dynamic())
	assert.Equal(t, 4, ep.IPLookupFamily)
	require.Len(t, ep.IPLookupURLs, 2)
	assert.Empty(t, cfg.IPLookupURLs, "there are no top-level services")

	// Leader election, with the defaults
	assert.True(t, cfg.LeaderElection.Enabled)
	assert.Equal(t, "ddup", cfg.LeaderElection.Name)
	assert.Equal(t, 15*time.Second, cfg.LeaderElection.LeaseDuration)

	// Notifications: ntfy at a private in-cluster URL, with the token from a file, and email with the recipient from a file
	require.Len(t, cfg.Notifications.Webhooks, 1)
	ntfy := cfg.Notifications.Webhooks[0]
	assert.Equal(t, config.WebhookFormatNtfy, ntfy.Format)
	assert.Equal(t, "http://ntfy.system.svc.cluster.local:80/ddup", ntfy.URL)
	assert.Equal(t, ntfyToken, ntfy.Token)
	assert.True(t, ntfy.AllowPrivateDestinations)
	assert.Equal(t, 5, ntfy.MaxAttempts)
	assert.Equal(t, 2*time.Second, ntfy.RetryInterval)
	assert.Equal(t, 10*time.Second, ntfy.Timeout)
	require.Len(t, cfg.Notifications.Email, 1)
	email := cfg.Notifications.Email[0]
	assert.Equal(t, deploymentSecrets["EMAIL_CONNECTION_STRING"], email.ConnectionString)
	assert.Equal(t, []string{mailTo}, email.To)
	assert.Contains(t, email.ConnectionString, "tls=none")
}

// hostileConfigs are changes to the file that must make the configuration invalid, to be sure that the checks exist
func TestDeploymentLikeConfig_Rejections(t *testing.T) {
	tests := map[string]func(cfg *config.Config){
		"a lookup service over http": func(cfg *config.Config) {
			cfg.Domains[2].Endpoints[0].IPLookupURLs[0].URL = "http://lookup-a.example.com"
		},
		"a lookup endpoint that also has a fixed ip": func(cfg *config.Config) {
			cfg.Domains[2].Endpoints[0].IP = "192.0.2.1"
		},
		"a tunnel and a VPS with the same priority": func(cfg *config.Config) {
			cfg.Domains[0].Endpoints[2].Priority = 0
		},
		"an unknown notification event": func(cfg *config.Config) {
			cfg.Notifications.Webhooks[0].Events = []config.NotificationEvent{"nope"}
		},
		"a leader election with the durations in the wrong order": func(cfg *config.Config) {
			cfg.LeaderElection.RenewDeadline = 20 * time.Second
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := loadDeploymentConfig(t)
			mutate(cfg)
			require.Error(t, cfg.Validate(slog.Default()))
		})
	}
}

// fakeZone is the Cloudflare zone: it keeps the records of every domain, shared by the replicas, and counts the calls of each of them
type fakeZone struct {
	lock    sync.Mutex
	records map[string][]dns.Target
	// Number of calls by replica and domain
	calls map[string]map[string]int
	// Error to return for a domain
	fail map[string]error
}

func newFakeZone() *fakeZone {
	return &fakeZone{
		records: map[string][]dns.Target{},
		calls:   map[string]map[string]int{},
		fail:    map[string]error{},
	}
}

func sortedTargets(targets []dns.Target) []dns.Target {
	res := slices.Clone(targets)
	slices.SortFunc(res, func(a, b dns.Target) int {
		return strings.Compare(a.String(), b.String())
	})
	return res
}

// records returns the targets in the zone for a domain, sorted
func (z *fakeZone) get(domain string) []dns.Target {
	z.lock.Lock()
	defer z.lock.Unlock()
	return slices.Clone(z.records[domain])
}

// setFailure makes the updates of a domain fail with the error, or work again if it's nil
func (z *fakeZone) setFailure(domain string, err error) {
	z.lock.Lock()
	defer z.lock.Unlock()
	if err == nil {
		delete(z.fail, domain)
		return
	}
	z.fail[domain] = err
}

func (z *fakeZone) callsBy(replica, domain string) int {
	z.lock.Lock()
	defer z.lock.Unlock()
	return z.calls[replica][domain]
}

func (z *fakeZone) totalCallsBy(replica string) int {
	z.lock.Lock()
	defer z.lock.Unlock()
	var total int
	for _, n := range z.calls[replica] {
		total += n
	}
	return total
}

// zoneProvider is the DNS provider of one replica, writing to the shared zone
type zoneProvider struct {
	zone    *fakeZone
	replica string
	name    string
	caps    dns.Capabilities
}

func (p *zoneProvider) Name() string                   { return p.name }
func (p *zoneProvider) Capabilities() dns.Capabilities { return p.caps }

func (p *zoneProvider) UpdateRecords(_ context.Context, domain string, _ int, targets []dns.Target) (dns.UpdateResult, error) {
	z := p.zone
	z.lock.Lock()
	defer z.lock.Unlock()

	if z.calls[p.replica] == nil {
		z.calls[p.replica] = map[string]int{}
	}
	z.calls[p.replica][domain]++

	err := z.fail[domain]
	if err != nil {
		return dns.UpdateResult{}, err
	}

	want := sortedTargets(targets)
	prev := z.records[domain]
	res := dns.UpdateResult{Changed: !slices.Equal(prev, want)}
	for _, t := range prev {
		res.Previous = append(res.Previous, t.Value)
	}
	z.records[domain] = want
	return res, nil
}

// teeNotifier records the events, and passes them on to the real notifier
type teeNotifier struct {
	recordingNotifier

	next *notify.Notifier
}

func (n *teeNotifier) Notify(ev notify.Event) {
	n.recordingNotifier.Notify(ev)
	n.next.Notify(ev)
}

func (n *teeNotifier) eventsFor(domain string, typ config.NotificationEvent) []notify.Event {
	var res []notify.Event
	for _, ev := range n.Events() {
		if ev.Domain == domain && ev.Type == typ {
			res = append(res, ev)
		}
	}
	return res
}

// healthEndpoint is a server that answers the health checks: 418 when it's up, as the deployed probes expect, and 503 when it isn't
type healthEndpoint struct {
	*httptest.Server

	up atomic.Bool
}

func newHealthEndpoint(t *testing.T) *healthEndpoint {
	t.Helper()
	h := &healthEndpoint{}
	h.up.Store(true)
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if h.up.Load() {
			w.WriteHeader(http.StatusTeapot)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(h.Close)
	return h
}

// lookupService answers with the address it's told to, or fails
type lookupService struct {
	*httptest.Server

	address atomic.Value
	failing atomic.Bool
}

func newLookupService(t *testing.T, address string) *lookupService {
	t.Helper()
	s := &lookupService{}
	s.address.Store(address)
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.failing.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		address, _ := s.address.Load().(string)
		_, _ = io.WriteString(w, address+"\n")
	}))
	t.Cleanup(s.Close)
	return s
}

// receivedNtfy is a request that the fake ntfy server got
type receivedNtfy struct {
	path          string
	authorization string
	title         string
	priority      string
	tags          string
	body          string
}

type fakeNtfy struct {
	*httptest.Server

	lock     sync.Mutex
	requests []receivedNtfy
}

func newFakeNtfy(t *testing.T) *fakeNtfy {
	t.Helper()
	n := &fakeNtfy{}
	n.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		n.lock.Lock()
		n.requests = append(n.requests, receivedNtfy{
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
			title:         r.Header.Get("Title"),
			priority:      r.Header.Get("Priority"),
			tags:          r.Header.Get("Tags"),
			body:          string(body),
		})
		n.lock.Unlock()
		_, _ = io.WriteString(w, `{"id":"abc","time":1,"event":"message"}`)
	}))
	t.Cleanup(n.Close)
	return n
}

func (n *fakeNtfy) received() []receivedNtfy {
	n.lock.Lock()
	defer n.lock.Unlock()
	return slices.Clone(n.requests)
}

// fakeRelay is a mail relay without TLS and without authentication, like the one in the cluster
type fakeRelay struct {
	net.Listener

	lock     sync.Mutex
	messages []relayMessage
}

type relayMessage struct {
	from string
	to   []string
	data string
}

func newFakeRelay(t *testing.T) *fakeRelay {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	r := &fakeRelay{Listener: l}
	t.Cleanup(func() { _ = l.Close() })

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go r.serve(conn)
		}
	}()
	return r
}

func (r *fakeRelay) serve(conn net.Conn) {
	defer conn.Close() //nolint:errcheck
	br := bufio.NewReader(conn)
	reply := func(s string) { _, _ = io.WriteString(conn, s+"\r\n") }

	var msg relayMessage
	reply("220 relay.example.com ESMTP")
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"):
			// No STARTTLS and no AUTH, like the relay with "tls off" and no authentication
			reply("250-relay.example.com")
			reply("250 8BITMIME")
		case strings.HasPrefix(cmd, "HELO"):
			reply("250 relay.example.com")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			msg = relayMessage{from: strings.TrimSpace(line[len("MAIL FROM:"):])}
			reply("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			msg.to = append(msg.to, strings.TrimSpace(line[len("RCPT TO:"):]))
			reply("250 OK")
		case cmd == "DATA":
			reply("354 Go ahead")
			var data strings.Builder
			for {
				l, err := br.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				data.WriteString(l)
			}
			msg.data = data.String()
			r.lock.Lock()
			r.messages = append(r.messages, msg)
			r.lock.Unlock()
			reply("250 Queued")
		case cmd == "QUIT":
			reply("221 Bye")
			return
		case cmd == "RSET" || cmd == "NOOP":
			reply("250 OK")
		default:
			reply("502 Not implemented")
		}
	}
}

func (r *fakeRelay) received() []relayMessage {
	r.lock.Lock()
	defer r.lock.Unlock()
	return slices.Clone(r.messages)
}

// trust returns the options that make the lookups trust the certificate of the test servers
func trust(srv *httptest.Server) iplookup.Options {
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return iplookup.Options{TLSConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}
}

// replica is an instance of ddup, with its elector and its own DNS provider
type replica struct {
	hc      *HealthChecker
	elector leader.Elector
	stop    context.CancelFunc
	stopped chan struct{}
}

// Names of the domains of the file, and the targets they publish
var (
	ingressDomain = "ingress-lb.example.com"
	fastDomain    = "fast-lb.example.com"
	homeDomain    = ddnsName

	vpsUSTarget = dns.Target{Type: "A", Value: "192.0.2.10"}
	vpsEUTarget = dns.Target{Type: "A", Value: "192.0.2.11"}
	vpsTargets  = sortedTargets([]dns.Target{vpsUSTarget, vpsEUTarget})
	tunnelCNAME = tunnelTarget
	homeTarget  = dns.Target{Type: "A", Value: "203.0.113.45"}
	homeTarget2 = dns.Target{Type: "A", Value: "203.0.113.46"}
)

func only(targets ...dns.Target) []dns.Target {
	return sortedTargets(targets)
}

// deployment is two replicas of ddup running the configuration of the file, and the services around them
type deployment struct {
	ctx      context.Context //nolint:containedctx // Held by a test fixture
	cfg      *config.Config
	zone     *fakeZone
	tee      *teeNotifier
	notifier *notify.Notifier

	vpsUS, vpsEU, tunnel *healthEndpoint
	lookupA, lookupB     *lookupService
	ntfy                 *fakeNtfy
	relay                *fakeRelay

	client *fake.Clientset
	a, b   *replica
}

func newDeployment(t *testing.T) *deployment {
	t.Helper()

	d := &deployment{
		ctx:     t.Context(),
		cfg:     loadDeploymentConfig(t),
		zone:    newFakeZone(),
		vpsUS:   newHealthEndpoint(t),
		vpsEU:   newHealthEndpoint(t),
		tunnel:  newHealthEndpoint(t),
		lookupA: newLookupService(t, homeTarget.Value),
		lookupB: newLookupService(t, homeTarget.Value),
		ntfy:    newFakeNtfy(t),
		relay:   newFakeRelay(t),
		client:  fake.NewSimpleClientset(),
	}
	cfg := d.cfg

	// Point the health checks, the lookups and the notifications to the fakes
	// The structure of the configuration, which has been validated, is untouched
	for _, dom := range cfg.Domains {
		for _, ep := range dom.Endpoints {
			switch ep.Name {
			case "vps-us":
				ep.URL = d.vpsUS.URL
			case "vps-eu":
				ep.URL = d.vpsEU.URL
			case "tunnel":
				ep.URL = d.tunnel.URL
			case "home":
				ep.IPLookupURLs[0].URL = d.lookupA.URL
				ep.IPLookupURLs[1].URL = d.lookupB.URL
			}
		}
	}
	cfg.Notifications.Webhooks[0].URL = d.ntfy.URL + "/ddup"
	cfg.Notifications.Email[0].ConnectionString = strings.Replace(cfg.Notifications.Email[0].ConnectionString, relayAddr, d.relay.Addr().String(), 1)

	// The notifier is created from the configuration, as in main
	var err error
	d.notifier, err = notify.New(d.ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, d.notifier)
	d.tee = &teeNotifier{next: d.notifier}

	// The first replica becomes the leader, and the second one is a standby
	d.a = d.newReplica(t, "1")
	require.Eventually(t, d.a.elector.IsLeader, 5*time.Second, 20*time.Millisecond, "the first replica becomes the leader")
	d.b = d.newReplica(t, "2")

	return d
}

// newReplica starts a replica, with its own state and the same Lease as the others
func (d *deployment) newReplica(t *testing.T, name string) *replica {
	t.Helper()

	// The providers can be created from the configuration (credentials from files), and the zone stands in for them
	providers := map[string]dns.Provider{}
	for pname, pc := range d.cfg.Providers {
		p, err := dns.NewProvider(pname, &pc, nil)
		require.NoError(t, err)
		providers[pname] = &zoneProvider{zone: d.zone, replica: name, name: pname, caps: p.Capabilities()}
	}

	hc, err := newHealthChecker(providers, nil, trust(d.lookupA.Server))
	require.NoError(t, err)
	hc.SetNotifier(d.tee)

	le := d.cfg.LeaderElection
	elector, err := leader.NewKube(leader.KubeOpts{
		Namespace:     "networking",
		Name:          le.Name,
		Identity:      "10.0.0." + name,
		LeaseDuration: le.LeaseDuration,
		RenewDeadline: le.RenewDeadline,
		RetryPeriod:   le.RetryPeriod,
		Client:        d.client,
	})
	require.NoError(t, err)
	hc.SetElector(elector)

	runCtx, cancel := context.WithCancel(d.ctx)
	r := &replica{hc: hc, elector: elector, stop: cancel, stopped: make(chan struct{})}
	go func() {
		defer close(r.stopped)
		_ = elector.Run(runCtx)
	}()
	t.Cleanup(func() {
		cancel()
		<-r.stopped
	})
	return r
}

// cycle runs n cycles of checks in both replicas
func (d *deployment) cycle(n int) {
	for range n {
		d.a.hc.checkAndUpdateDNS(d.ctx)
		d.b.hc.checkAndUpdateDNS(d.ctx)
	}
}

func (d *deployment) setVPS(up bool) {
	d.vpsUS.up.Store(up)
	d.vpsEU.up.Store(up)
}

func (d *deployment) stageStart(t *testing.T) {
	d.cycle(1)
	assert.Equal(t, vpsTargets, d.zone.get(ingressDomain), "both VPS IPs as A records that are not proxied, and the tunnel on standby")
	assert.Equal(t, only(tunnelCNAME), d.zone.get(fastDomain), "the proxied tunnel CNAME alone, and the VPS on standby")
	assert.Equal(t, only(homeTarget), d.zone.get(homeDomain), "the dynamic record follows the looked-up address, in the other zone's provider")

	assert.Equal(t, 1, d.zone.callsBy("1", ingressDomain))
	assert.Equal(t, 1, d.zone.callsBy("1", fastDomain))
	assert.Equal(t, 1, d.zone.callsBy("1", homeDomain))
	assert.Zero(t, d.zone.totalCallsBy("2"), "the standby does not touch DNS")

	for _, domain := range []string{ingressDomain, fastDomain, homeDomain} {
		assert.Len(t, d.tee.eventsFor(domain, config.EventDNSUpdated), 1, domain)
	}

	// The status shows the tiers
	status := d.a.hc.GetDomainStatus(ingressDomain)
	require.NotNil(t, status)
	byName := map[string]DomainStatusEndpoint{}
	for _, e := range status.Endpoints {
		byName[e.Name] = e
	}
	assert.True(t, byName["vps-us"].Active)
	assert.True(t, byName["vps-eu"].Active)
	assert.False(t, byName["tunnel"].Active)
	assert.True(t, byName["tunnel"].Healthy, "healthy on standby")
	assert.Equal(t, "CNAME", byName["tunnel"].Type)
	assert.True(t, byName["tunnel"].Proxied)
	assert.Equal(t, 1, byName["tunnel"].Priority)

	// The standby sees the same, from its own checks
	standby := d.b.hc.GetDomainStatus(ingressDomain)
	require.NotNil(t, standby)
	assert.Len(t, standby.Endpoints, 3)
}

func (d *deployment) stageVPSFail(t *testing.T) {
	d.setVPS(false)
	d.cycle(2)
	assert.Equal(t, vpsTargets, d.zone.get(ingressDomain), "two failures out of three attempts change nothing")
	assert.Equal(t, 1, d.zone.callsBy("1", ingressDomain))

	d.cycle(1)
	assert.Equal(t, only(tunnelCNAME), d.zone.get(ingressDomain), "the proxied CNAME tunnel is published after the IPs go down")
	assert.Equal(t, only(tunnelCNAME), d.zone.get(fastDomain), "the other domain, which already had the tunnel, is untouched")
	assert.Equal(t, 1, d.zone.callsBy("1", fastDomain))
	updates := d.tee.eventsFor(ingressDomain, config.EventDNSUpdated)
	require.Len(t, updates, 2)
	assert.Equal(t, []string{tunnelCNAME.Value}, updates[1].Published, "the notification names the CNAME hostname")
	assert.Zero(t, d.zone.totalCallsBy("2"))
}

func (d *deployment) stageVPSRecover(t *testing.T) {
	d.setVPS(true)
	d.cycle(2)
	assert.Equal(t, only(tunnelCNAME), d.zone.get(ingressDomain), "still recovering")
	d.cycle(1)
	assert.Equal(t, vpsTargets, d.zone.get(ingressDomain))
	assert.Len(t, d.tee.eventsFor(ingressDomain, config.EventDNSUpdated), 3)
}

func (d *deployment) stageTunnelFail(t *testing.T) {
	d.tunnel.up.Store(false)
	d.cycle(2)
	assert.Equal(t, only(tunnelCNAME), d.zone.get(fastDomain))
	d.cycle(1)
	assert.Equal(t, vpsTargets, d.zone.get(fastDomain), "the VPS A records are published when the tunnel is down")
	assert.Equal(t, vpsTargets, d.zone.get(ingressDomain), "ingress-lb keeps its IPs: the tunnel was only its fallback")

	d.tunnel.up.Store(true)
	d.cycle(2)
	assert.Equal(t, vpsTargets, d.zone.get(fastDomain))
	d.cycle(1)
	assert.Equal(t, only(tunnelCNAME), d.zone.get(fastDomain), "back to the tunnel")
}

func (d *deployment) stageHomeAddress(t *testing.T) {
	d.lookupA.address.Store(homeTarget2.Value)
	d.lookupB.address.Store(homeTarget2.Value)
	d.cycle(1)
	assert.Equal(t, only(homeTarget2), d.zone.get(homeDomain))

	// One service fails: the other one answers
	d.lookupA.failing.Store(true)
	d.cycle(1)
	assert.Equal(t, only(homeTarget2), d.zone.get(homeDomain))
	d.lookupA.failing.Store(false)
}

func (d *deployment) stageLeaderChange(t *testing.T) {
	before := len(d.tee.Events())
	callsBefore := d.zone.totalCallsBy("1")
	require.Positive(t, callsBefore)

	// The first replica stops, like a rolling update, and hands the Lease back
	d.a.stop()
	<-d.a.stopped
	require.Eventually(t, d.b.elector.IsLeader, 15*time.Second, 50*time.Millisecond, "the standby takes over")
	assert.False(t, d.a.elector.IsLeader())

	d.cycle(1)
	assert.Equal(t, 1, d.zone.callsBy("2", ingressDomain), "the new leader asserts the records of each domain")
	assert.Equal(t, 1, d.zone.callsBy("2", fastDomain))
	assert.Equal(t, 1, d.zone.callsBy("2", homeDomain))
	assert.Equal(t, callsBefore, d.zone.totalCallsBy("1"), "the old leader does not touch DNS any more")
	assert.Equal(t, vpsTargets, d.zone.get(ingressDomain))
	assert.Equal(t, only(tunnelCNAME), d.zone.get(fastDomain))
	assert.Equal(t, only(homeTarget2), d.zone.get(homeDomain))
	assert.Len(t, d.tee.Events(), before, "the records were already right, so there's nothing to tell")

	// And it keeps going: no more calls while nothing changes
	d.cycle(2)
	assert.Equal(t, 1, d.zone.callsBy("2", ingressDomain))
}

func (d *deployment) stageDNSFailure(t *testing.T) {
	d.zone.setFailure(ingressDomain, assert.AnError)

	d.setVPS(false)
	d.cycle(3)
	require.Len(t, d.tee.eventsFor(ingressDomain, config.EventDNSUpdateFailed), 1)
	d.cycle(1)
	assert.Len(t, d.tee.eventsFor(ingressDomain, config.EventDNSUpdateFailed), 1, "not repeated while the error is the same")

	// The zone works again: the update that failed is retried and published
	d.zone.setFailure(ingressDomain, nil)
	d.cycle(1)
	assert.Equal(t, only(tunnelCNAME), d.zone.get(ingressDomain))

	d.setVPS(true)
	d.cycle(3)
	assert.Equal(t, vpsTargets, d.zone.get(ingressDomain))
}

func (d *deployment) stageEverythingFails(t *testing.T) {
	domains := []string{ingressDomain, fastDomain, homeDomain}
	recordsBefore := map[string][]dns.Target{}
	for _, domain := range domains {
		recordsBefore[domain] = d.zone.get(domain)
	}
	callsBefore := d.zone.totalCallsBy("2")

	d.setVPS(false)
	d.tunnel.up.Store(false)
	d.lookupA.failing.Store(true)
	d.lookupB.failing.Store(true)
	d.cycle(2)
	for _, domain := range domains {
		assert.Empty(t, d.tee.eventsFor(domain, config.EventAllUnhealthy), domain)
	}
	d.cycle(1)
	for _, domain := range domains {
		assert.Len(t, d.tee.eventsFor(domain, config.EventAllUnhealthy), 1, domain)
	}
	d.cycle(3)
	for _, domain := range domains {
		assert.Len(t, d.tee.eventsFor(domain, config.EventAllUnhealthy), 1, "%s: announced once", domain)
		assert.Equal(t, recordsBefore[domain], d.zone.get(domain), "%s: left as it was", domain)
	}
	assert.Equal(t, callsBefore, d.zone.totalCallsBy("2"), "no DNS call without a healthy endpoint")

	// One endpoint comes back: that is what DNS points to
	d.vpsEU.up.Store(true)
	d.cycle(3)
	assert.Equal(t, only(vpsEUTarget), d.zone.get(ingressDomain))
	assert.Equal(t, only(vpsEUTarget), d.zone.get(fastDomain))
}

// checkDestinations delivers what is in the queue, then looks at what ntfy and the relay got
func (d *deployment) checkDestinations(t *testing.T) {
	t.Helper()

	shutdownCtx, cancel := context.WithTimeout(d.ctx, 20*time.Second)
	defer cancel()
	require.NoError(t, d.notifier.Shutdown(shutdownCtx))

	events := d.tee.Events()
	require.NotEmpty(t, events)
	var outages int
	for _, ev := range events {
		if ev.Type == config.EventAllUnhealthy {
			outages++
		}
	}
	require.Equal(t, 3, outages)

	t.Run("ntfy got every event, with the token and the priority", func(t *testing.T) {
		got := d.ntfy.received()
		require.Len(t, got, len(events), "one request for each event")

		var urgent int
		for _, req := range got {
			assert.Equal(t, "Bearer "+ntfyToken, req.authorization, "the token comes from the file")
			if req.priority == "5" || req.priority == "max" || req.priority == "urgent" {
				urgent++
				assert.Contains(t, req.title, "No healthy endpoints for ")
				assert.Contains(t, req.tags, "red_circle")
			}
		}
		assert.Equal(t, outages, urgent, "an outage is urgent (priority 5)")
	})

	t.Run("email reached the relay without TLS, for the recipient in the file", func(t *testing.T) {
		got := d.relay.received()
		require.Len(t, got, len(events), "one email for each event")
		for _, m := range got {
			assert.Contains(t, m.from, "ddup@example.com")
			assert.Equal(t, []string{"<" + mailTo + ">"}, m.to)
			assert.Contains(t, m.data, "Subject:")
		}
	})
}

// The stages depend on the ones before them
func TestDeploymentLike_Cycles(t *testing.T) {
	d := newDeployment(t)

	t.Run("start: the first tier of every domain is published, by the leader only", d.stageStart)
	t.Run("both VPS fail: the tunnel is published after the attempts, and not before", d.stageVPSFail)
	t.Run("the VPS recover: the IPs are back after recoverAfter successful checks", d.stageVPSRecover)
	t.Run("the tunnel fails: fast-lb falls back to the VPS IPs", d.stageTunnelFail)
	t.Run("the address of the home network changes: the record follows it", d.stageHomeAddress)
	t.Run("the leader changes: the new one asserts every record, and nobody is notified again", d.stageLeaderChange)
	t.Run("a DNS failure is reported once", d.stageDNSFailure)
	t.Run("everything fails: each domain announces the outage once, and DNS is left alone", d.stageEverythingFails)
	d.checkDestinations(t)
}

// The deployed ntfy is at a private address, so the notification needs allowPrivateDestinations
func TestDeploymentLike_NtfyNeedsAllowPrivateDestinations(t *testing.T) {
	for _, allow := range []bool{true, false} {
		t.Run(map[bool]string{true: "allowed", false: "not allowed"}[allow], func(t *testing.T) {
			cfg := loadDeploymentConfig(t)
			ntfy := newFakeNtfy(t)
			cfg.Notifications.Webhooks[0].URL = ntfy.URL + "/ddup"
			cfg.Notifications.Webhooks[0].AllowPrivateDestinations = allow
			cfg.Notifications.Webhooks[0].MaxAttempts = 1
			cfg.Notifications.Email = nil

			notifier, err := notify.New(t.Context(), cfg)
			require.NoError(t, err)
			notifier.Notify(notify.Event{
				Type:   config.EventAllUnhealthy,
				Domain: "ingress-lb.example.com",
				Endpoints: []notify.EndpointState{
					{Name: "vps-us", IP: "192.0.2.10"},
				},
			})
			require.NoError(t, notifier.Shutdown(t.Context()))

			if allow {
				require.Len(t, ntfy.received(), 1)
			} else {
				assert.Empty(t, ntfy.received())
			}
		})
	}
}
