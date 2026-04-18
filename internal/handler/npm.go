package handler

import (
	"context"
	"path/filepath"
	"strings"

	"cxcdn/internal/cache"
	"cxcdn/internal/registry"
	"cxcdn/internal/storage"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type cachedResponse struct {
	Status  int
	Headers map[string]string
	Body    []byte
}

// NpmFile handles /npm/{package}(@{version})/{file}
func NpmFile(ctx context.Context, c *app.RequestContext) {
	pkgInput := c.Param("package")
	filePath := c.Param("file")

	// Strip leading slash from file path (Hertz wildcard includes leading /)
	filePath = strings.TrimPrefix(filePath, "/")
	if filePath == "" {
		NpmList(ctx, c)
		return
	}

	name, versionReq := registry.ParsePackageName(pkgInput)

	version, tarball, err := registry.ResolveNpmVersion(name, versionReq)
	if err != nil {
		c.String(consts.StatusNotFound, "Error: %v", err)
		return
	}

	data, err := storage.GetFileFromPackage(name, version, tarball, filePath)
	if err != nil {
		c.String(consts.StatusNotFound, "File not found: %s", filePath)
		return
	}

	contentType := mimeFromExt(filePath)
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Data(consts.StatusOK, contentType, data)
}

// NpmList handles /npm/{package}(@{version})/
func NpmList(ctx context.Context, c *app.RequestContext) {
	pkgInput := c.Param("package")

	// Try response cache first
	cacheKey := "npm:page:" + pkgInput
	if cached, found := cache.C.Get(cacheKey); found {
		resp := cached.(*cachedResponse)
		for k, v := range resp.Headers {
			c.Header(k, v)
		}
		c.Data(resp.Status, "text/html; charset=utf-8", resp.Body)
		return
	}

	name, versionReq := registry.ParsePackageName(pkgInput)

	version, tarball, err := registry.ResolveNpmVersion(name, versionReq)
	if err != nil {
		c.String(consts.StatusNotFound, "Error: %v", err)
		return
	}

	files, err := storage.ListPackageFiles(name, version, tarball)
	if err != nil {
		c.String(consts.StatusInternalServerError, "Error listing files: %v", err)
		return
	}

	// Get all versions for the selector
	allVersions, latest, _ := registry.GetNpmVersions(name)
	var versionOptions []VersionOption
	for _, v := range allVersions {
		versionOptions = append(versionOptions, VersionOption{
			Version: v,
			URL:     "/npm/" + name + "@" + v + "/",
			Current: v == version,
		})
	}
	// Mark latest
	if latest != "" {
		for i := range versionOptions {
			if versionOptions[i].Version == latest {
				versionOptions[i].Version = latest + " (latest)"
				break
			}
		}
	}

	pkgDisplayName := name + "@" + version
	html := renderFileListHTML("npm", pkgDisplayName, files, "/npm/"+name+"@"+version+"/", versionOptions)

	// Cache the rendered HTML
	cache.C.Set(cacheKey, &cachedResponse{
		Status: 200,
		Headers: map[string]string{
			"Cache-Control":          "public, max-age=300",
			"Access-Control-Allow-Origin": "*",
		},
		Body: html,
	}, cache.DefaultExpiration)

	c.Header("Cache-Control", "public, max-age=300")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Data(200, "text/html; charset=utf-8", html)
}

// mimeFromExt returns a MIME type based on file extension
func mimeFromExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".mjs":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	case ".map":
		return "application/json"
	case ".ts":
		return "text/typescript; charset=utf-8"
	case ".wasm":
		return "application/wasm"
	case ".xml":
		return "application/xml; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".md":
		return "text/markdown; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
