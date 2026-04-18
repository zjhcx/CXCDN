package server

import (
	"context"
	"log"
	"time"

	"cxcdn/internal/cache"
	"cxcdn/internal/handler"
	"cxcdn/internal/pool"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// RunHertz starts the CDN server using Hertz (high-performance)
func RunHertz(addr string, cacheFile string) error {
	cache.Init(cacheFile)

	h := server.New(
		server.WithHostPorts(addr),
		server.WithTransport(standard.NewTransporter),
		server.WithReadBufferSize(32*1024),
		server.WithIdleTimeout(60*time.Second),
		server.WithNetwork("tcp"),
	)

	// Index page
	h.GET("/", handler.RenderIndexHandler)

	// NPM routes
	h.GET("/npm/:package/*file", handler.NpmFile)

	// GitHub routes
	h.GET("/gh/:user/:repo/*file", handler.GhFile)

	// Stats endpoint
	h.GET("/_stats", func(ctx context.Context, c *app.RequestContext) {
		stats := pool.DefaultPool.Stats()
		c.JSON(consts.StatusOK, map[string]interface{}{
			"pooled_hosts": len(stats),
			"hosts":        stats,
		})
	})

	log.Printf("[hertz] CXCDN starting on %s (high-performance mode)", addr)
	log.Printf("[pool] Connection pool initialized: maxIdle=%d, maxPerHost=%d", 100, 20)
	if cacheFile != "" {
		log.Printf("[cache] Disk persistence enabled: %s", cacheFile)
	}

	// Graceful shutdown: save cache on exit
	h.OnShutdown = append(h.OnShutdown, func(ctx context.Context) {
		log.Printf("[cache] Shutting down, saving cache to disk...")
		cache.Stop()
	})

	h.Spin()
	return nil
}
