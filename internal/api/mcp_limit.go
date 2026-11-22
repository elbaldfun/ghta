package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// MCP traffic limits, sized for the 1 vCPU / 2 GB droplet: an agent session
// makes a handful of calls in a burst, then pauses.
const (
	mcpPerClientRate  = rate.Limit(1) // calls per second, sustained
	mcpPerClientBurst = 20
	mcpMaxInFlight    = 4 // concurrent tool calls across all clients
	mcpMaxBody        = 64 << 10
	mcpMaxClients     = 10000 // tracked limiters before the table is reset
	mcpClientIdle     = 10 * time.Minute
)

// MCPLimiter rate-limits /mcp by client address and caps the calls it
// serves at once. A request over either limit gets 429 at once rather than
// queueing on the small machine.
type MCPLimiter struct {
	mu        sync.Mutex
	clients   map[string]*mcpClient
	slots     chan struct{}
	rate      rate.Limit
	burst     int
	now       func() time.Time
	lastPrune time.Time
}

type mcpClient struct {
	lim  *rate.Limiter
	seen time.Time
}

func NewMCPLimiter() *MCPLimiter {
	return &MCPLimiter{clients: map[string]*mcpClient{}, slots: make(chan struct{}, mcpMaxInFlight),
		rate: mcpPerClientRate, burst: mcpPerClientBurst, now: time.Now}
}

func (l *MCPLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastPrune) > mcpClientIdle || len(l.clients) >= mcpMaxClients {
		for k, c := range l.clients {
			if now.Sub(c.seen) > mcpClientIdle || len(l.clients) >= mcpMaxClients {
				delete(l.clients, k)
			}
		}
		l.lastPrune = now
	}
	c := l.clients[key]
	if c == nil {
		c = &mcpClient{lim: rate.NewLimiter(l.rate, l.burst)}
		l.clients[key] = c
	}
	c.seen = now
	return c.lim.AllowN(now, 1)
}

// Wrap guards next with the limits.
func (l *MCPLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only POST carries calls; a stateless server answers the rest with
		// 405 itself, cheaply.
		if r.Method == http.MethodPost {
			if !l.allow(clientIP(r)) {
				tooMany(w, "rate limit exceeded; slow down")
				return
			}
			select {
			case l.slots <- struct{}{}:
				defer func() { <-l.slots }()
			default:
				tooMany(w, "server busy; retry shortly")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, mcpMaxBody)
		}
		next.ServeHTTP(w, r)
	})
}

func tooMany(w http.ResponseWriter, msg string) {
	w.Header().Set("Retry-After", "5")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32000,"message":"` + msg + `"}}`))
}

// clientIP is the caller's address. Behind the edge Caddy the peer is a
// private address and Caddy has set X-Forwarded-For to the real client (it
// discards what the client sent unless the client is a trusted proxy), so
// the last entry is taken; a direct public peer is used as it is.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsPrivate() || ip.IsLoopback()) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if last := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(last) != nil {
				return last
			}
		}
	}
	return host
}
