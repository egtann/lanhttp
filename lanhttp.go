package lanhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type Client struct {
	client HTTPClient
	log    *logger
	stop   chan struct{}

	// backends that are currently live
	backends backends

	// mu protects backends from concurrent access
	mu sync.RWMutex
}

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Logger is the public logging interface. We wrap this with our own logger to
// provide some more control.
type Logger interface{ Printf(string, ...interface{}) }

type Routes map[string][]string

type backends map[string]backend

type backend struct {
	IPs   []string
	Index int
}

type logger struct {
	l  Logger
	mu sync.RWMutex
}

func (l *logger) Printf(s string, vs ...interface{}) {
	// By default don't log
	if l.l == nil {
		return
	}

	// If we have a logger, then lock it to ensure we don't write while
	// it's being replaced. In practice we only log on errors so this
	// should have a negligible impact
	l.mu.RLock()
	defer l.mu.RUnlock()

	l.l.Printf(s, vs...)
}

func NewClient(client HTTPClient) *Client {
	return &Client{
		log:      &logger{},
		client:   client,
		backends: backends{},
		stop:     make(chan struct{}),
	}
}

// WithLogger replaces the logger of a client in a threadsafe way. This can be
// used for instance to load up the internal LAN clients immediately, then
// update the logger with new settings later in the program, e.g. after
// environment variables are loaded and you try to connect to internal
// services.
func (c *Client) WithLogger(lg Logger) *Client {
	c.log.mu.Lock()
	defer c.log.mu.Unlock()

	c.log = &logger{l: lg}
	return c
}

// UpdateRoutes in the client for internal servers. This can be called
// periodically based on healthchecks from an external service such as a
// reverse proxy. Unless you are manually updating your routes, you should use
// StartUpdating and StopUpdating instead.
func (c *Client) UpdateRoutes(new Routes) {
	// Check if routes have changed. Most of the time they have not, so we
	// don't need the write lock.
	if changed := diff(new, c.Routes()); !changed {
		return
	}
	c.changeRoutes(new)
}

func (c *Client) first(urls []string, timeout time.Duration) Routes {
	ch := make(chan Routes)
	update := func(uri string) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", uri, nil)
		if err != nil {
			c.log.Printf("%s: new request: %s", uri, err)
			return
		}
		resp, err := c.client.Do(req)
		if err != nil {
			c.log.Printf("%s: do: %s", uri, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			c.log.Printf("%s: bad status code: %d", uri, resp.StatusCode)
			return
		}
		routes := Routes{}
		if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
			c.log.Printf("%s: decode: %s", uri, err)
			return
		}
		ch <- routes
	}
	for _, uri := range urls {
		go update(uri)
	}
	select {
	case routes := <-ch:
		return routes
	case <-time.After(timeout):
		return Routes{}
	}
}

// StartUpdating live backends with an initial, synchronous update before
// continuing. Try all URLs simultaneously and use results from the first
// reply. Note that even when this fails, we still allow the code to
// continue... Just don't expect internal IPs to route until the servers come
// online.
func (c *Client) StartUpdating(urls []string, every time.Duration) {
	c.changeRoutes(c.first(urls, every))
	go func() {
		for {
			select {
			case <-time.After(every):
				c.changeRoutes(c.first(urls, every))
			case <-c.stop:
				return
			}
		}
	}()
}

func (c *Client) StopUpdating() {
	c.stop <- struct{}{}
}

func (c *Client) changeRoutes(new Routes) {
	if changed := diff(new, c.Routes()); !changed {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	backends := map[string]backend{}
	for k, ips := range new {
		backends[k] = backend{IPs: ips}
	}
	c.backends = backends
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	req.URL = c.ResolveHost(req.URL)
	return c.client.Do(req)
}

// ResolveHost from a URL to a specific IP if internal, otherwise return the
// URL unmodified.
func (c *Client) ResolveHost(uri *url.URL) *url.URL {
	host, port, err := net.SplitHostPort(uri.Host)
	if err != nil {
		host = uri.Host
		port = ""
	}
	if !strings.HasSuffix(host, ".internal") {
		return uri
	}
	ip := c.getIP(host)
	if ip == "" {
		return uri
	}
	if port == "" {
		uri.Host = ip
	} else {
		uri.Host = fmt.Sprintf("%s:%s", ip, port)
	}
	return uri
}

func (c *Client) getIP(host string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	backend, ok := c.backends[host]
	if !ok {
		return ""
	}
	if len(backend.IPs) == 0 {
		return ""
	}
	backend.Index = (backend.Index + 1) % len(backend.IPs)
	return backend.IPs[backend.Index]
}

func (c *Client) Routes() Routes {
	c.mu.RLock()
	defer c.mu.RUnlock()

	r := make(Routes, len(c.backends))
	for k, v := range c.backends {
		r[k] = append([]string{}, v.IPs...)
	}
	return r
}

func diff(a, b Routes) bool {
	for key := range a {
		// Exit quickly if lengths are different
		if len(a[key]) != len(b[key]) {
			return true
		}

		// Sort the live backends to get better performance when
		// diffing them
		sort.Slice(a[key], func(i, j int) bool {
			return a[key][i] < a[key][j]
		})
		sort.Slice(b[key], func(i, j int) bool {
			return b[key][i] < b[key][j]
		})

		// Compare two and exit on the first different string
		for i, ip := range a[key] {
			if b[key][i] != ip {
				return true
			}
		}
	}
	return false
}
