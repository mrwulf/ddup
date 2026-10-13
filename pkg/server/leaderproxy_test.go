package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/italypaleale/ddup/pkg/buildinfo"
	"github.com/italypaleale/ddup/pkg/config"
)

type fakeElector struct {
	leader   bool
	leaderID string
}

func (*fakeElector) Run(ctx context.Context) error { <-ctx.Done(); return nil }
func (f *fakeElector) IsLeader() bool              { return f.leader }
func (f *fakeElector) Leader() string              { return f.leaderID }

// startLeader starts a fake leader on 127.0.0.1 and points the config at its port
func startLeader(t *testing.T, handler http.Handler) {
	t.Helper()

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	_, portStr, err := net.SplitHostPort(ts.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	cfg := config.Get()
	prev := cfg.Server.Port
	cfg.Server.Port = port
	t.Cleanup(func() { cfg.Server.Port = prev })
}

func serve(t *testing.T, el *fakeElector, method string, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	s, err := NewServer(NewServerOpts{HealthChecker: &fakeProvider{}, Elector: el})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func TestLeaderProxy(t *testing.T) {
	var got *http.Request
	leader := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("X-From", "leader")
		_, _ = w.Write([]byte(`{"from":"leader"}`))
	})

	t.Run("a standby forwards API requests to the leader", func(t *testing.T) {
		got = nil
		startLeader(t, leader)

		rec := serve(t, &fakeElector{leaderID: "127.0.0.1"}, http.MethodPost, "/api/check", map[string]string{headerRequestedBy: requestedByValue})
		assert.Equal(t, "leader", rec.Header().Get("X-From"))
		require.NotNil(t, got)
		assert.Equal(t, "/api/check", got.URL.Path)
		assert.Equal(t, requestedByValue, got.Header.Get(headerRequestedBy), "the CSRF header must reach the leader")
		assert.Equal(t, "1", got.Header.Get(headerForwarded))
	})

	t.Run("the leader answers itself", func(t *testing.T) {
		got = nil
		startLeader(t, leader)

		rec := serve(t, &fakeElector{leader: true, leaderID: "127.0.0.1"}, http.MethodGet, "/api/info", nil)
		assert.Nil(t, got)
		assert.Empty(t, rec.Header().Get("X-From"))
		assert.Contains(t, rec.Body.String(), buildinfo.AppVersion)
	})

	t.Run("a request already forwarded is not forwarded again", func(t *testing.T) {
		got = nil
		startLeader(t, leader)

		rec := serve(t, &fakeElector{leaderID: "127.0.0.1"}, http.MethodGet, "/api/info", map[string]string{headerForwarded: "1"})
		assert.Nil(t, got)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("an unknown leader or one that isn't an IP is answered locally", func(t *testing.T) {
		got = nil
		startLeader(t, leader)

		for _, id := range []string{"", "ddup-abc", "evil.example.com"} {
			rec := serve(t, &fakeElector{leaderID: id}, http.MethodGet, "/api/info", nil)
			assert.Nil(t, got, "leader id %q", id)
			assert.Equal(t, http.StatusOK, rec.Code, "leader id %q", id)
			assert.Empty(t, rec.Header().Get("X-From"))
		}
	})

	t.Run("an unreachable leader falls back to a local answer", func(t *testing.T) {
		// Reserve a port, then close it so nothing listens there
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := l.Addr().(*net.TCPAddr).Port
		require.NoError(t, l.Close())

		cfg := config.Get()
		prev := cfg.Server.Port
		cfg.Server.Port = port
		t.Cleanup(func() { cfg.Server.Port = prev })

		rec := serve(t, &fakeElector{leaderID: "127.0.0.1"}, http.MethodGet, "/api/info", nil)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), buildinfo.AppVersion)
	})

	t.Run("health checks are never forwarded", func(t *testing.T) {
		got = nil
		startLeader(t, leader)

		rec := serve(t, &fakeElector{leaderID: "127.0.0.1"}, http.MethodGet, "/healthz", nil)
		assert.Nil(t, got)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
