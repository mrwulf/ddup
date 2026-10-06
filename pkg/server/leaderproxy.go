package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"

	"github.com/italypaleale/ddup/pkg/config"
)

const (
	// Set on requests forwarded to the leader, so a request is never forwarded twice
	headerForwarded = "X-Ddup-Forwarded"

	// Forced checks take a while, so the leader has more than forceCheckTimeout to answer
	leaderResponseTimeout = forceCheckTimeout + 5*time.Second
)

type leaderAddrKey struct{}

// leaderAware returns a function that registers an API route on the mux
// On a standby, the request is forwarded to the leader, so everyone sees the leader's view and "check now" runs where it matters
// The request is answered locally when this instance is the leader, when there's no election, when the leader is unknown, or when the leader can't be reached
func (s *Server) leaderAware(mux *http.ServeMux) func(pattern string, handler http.HandlerFunc) {
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
		ResponseHeaderTimeout: leaderResponseTimeout,
	}

	return func(pattern string, handler http.HandlerFunc) {
		proxy := &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				addr, _ := pr.In.Context().Value(leaderAddrKey{}).(string)
				pr.Out.URL.Scheme = "http"
				pr.Out.URL.Host = addr
				pr.Out.Host = addr
				pr.Out.Header.Set(headerForwarded, "1")
			},
			Transport: transport,
			// Runs when the leader is unreachable: fall back to our own answer
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				slog.WarnContext(r.Context(), "Failed to forward request to the leader, answering locally", slog.Any("error", err))
				handler(w, r)
			},
		}

		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			addr := s.leaderAddr(r)
			if addr == "" {
				handler(w, r)
				return
			}

			proxy.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), leaderAddrKey{}, addr)))
		})
	}
}

// leaderAddr returns the address of the leader if this request should be forwarded there, or an empty string if it should be answered locally
func (s *Server) leaderAddr(r *http.Request) string {
	if s.elector == nil || s.elector.IsLeader() || r.Header.Get(headerForwarded) != "" {
		return ""
	}

	// The leader's identity is its pod IP
	// Anything else is not an address we can forward to, and we never forward to a name we didn't expect
	id := s.elector.Leader()
	if net.ParseIP(id) == nil {
		return ""
	}

	return net.JoinHostPort(id, strconv.Itoa(config.Get().Server.Port))
}
