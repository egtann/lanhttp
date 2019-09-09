package lanhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Client struct {
	log    Logger
	client HTTPClient
	stop   chan struct{}

	// backends that are currently live
	backends backends

	// mu protects backends from concurrent access
	mu sync.RWMutex
}

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Logger interface {
	Printf(string, ...interface{})
}

type Routes map[string][]string

type backends map[string]backend

type backend struct {
	IPs   []string
	Index int
}

func New(client HTTPClient, lg Logger) *Client {
	return &Client{
		log:      lg,
		client:   client,
		backends: backends{},
		stop:     make(chan struct{}),
	}
}

// UpdateRoutes in the client for internal servers. This can be called
// periodically based on healthchecks from an external service such as a
// reverse proxy. Unless you are manually updating your routes, you should use
// StartUpdating and StopUpdating instead.
func (c *Client) UpdateRoutes(new Routes) {
	// Check if routes have changed. Most of the time they have not, so we
	// don't need the write lock.
	if changed := diff(new, c.routes()); !changed {
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
		if err := json.NewDecoder(resp.Body).Decode(routes); err != nil {
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
	if changed := diff(new, c.routes()); !changed {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	var backends map[string]backend
	for k, ips := range new {
		backends[k] = backend{IPs: ips}
	}
	c.backends = backends
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	host, port, err := net.SplitHostPort(req.URL.Host)
	if err != nil {
		return nil, fmt.Errorf("split host port: %w", err)
	}
	if !strings.HasSuffix(host, ".internal") {
		return c.client.Do(req)
	}
	ip := c.getIP(host)
	if ip == "" {
		return nil, fmt.Errorf("no live ip for host: %s", host)
	}
	req.URL.Host = fmt.Sprintf("%s:%s", ip, port)
	return c.client.Do(req)
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

func (c *Client) routes() Routes {
	c.mu.RLock()
	defer c.mu.RUnlock()

	r := Routes{}
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
