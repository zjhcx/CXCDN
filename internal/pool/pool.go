package pool

import (
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/network/standard"
)

// Pool manages reusable Hertz clients with persistent connections and connection pooling
type Pool struct {
	mu      sync.RWMutex
	clients map[string]*client.Client
}

var (
	DefaultPool *Pool
	once        sync.Once

	maxConnsPerHost     = 1024
	maxIdleConnDuration = 90 * time.Second
	dialTimeout         = 30 * time.Second
	connDuration        = 120 * time.Second
	readTimeout         = 120 * time.Second
	tarballConnDuration = 300 * time.Second
)

func init() {
	DefaultPool = NewPool()
}

// NewPool creates a new connection pool
func NewPool() *Pool {
	return &Pool{
		clients: make(map[string]*client.Client),
	}
}

// GetClient returns a Hertz client for the given host with optimized settings
func (p *Pool) GetClient(host string) *client.Client {
	p.mu.RLock()
	if c, ok := p.clients[host]; ok {
		p.mu.RUnlock()
		return c
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if c, ok := p.clients[host]; ok {
		return c
	}

	c, err := client.NewClient(
		client.WithDialer(standard.NewDialer()),
		client.WithDialTimeout(dialTimeout),
		client.WithMaxConnsPerHost(maxConnsPerHost),
		client.WithMaxIdleConnDuration(maxIdleConnDuration),
		client.WithMaxConnDuration(connDuration),
		client.WithClientReadTimeout(readTimeout),
	)
	if err != nil {
		panic(err)
	}

	p.clients[host] = c
	return c
}

// Get returns a client from the default pool
func Get(host string) *client.Client {
	return DefaultPool.GetClient(host)
}

// GetTarballClient returns a client with longer timeout for tarball downloads
func GetTarballClient() *client.Client {
	c, err := client.NewClient(
		client.WithDialer(standard.NewDialer()),
		client.WithDialTimeout(15*time.Second),
		client.WithMaxConnsPerHost(maxConnsPerHost),
		client.WithMaxIdleConnDuration(maxIdleConnDuration),
		client.WithMaxConnDuration(tarballConnDuration),
		client.WithClientReadTimeout(tarballConnDuration),
	)
	if err != nil {
		panic(err)
	}
	return c
}

// Stats returns pool statistics
func (p *Pool) Stats() map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]int)
	for host := range p.clients {
		stats[host] = 1
	}
	return stats
}
