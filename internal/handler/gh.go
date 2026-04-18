package handler

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"cxcdn/internal/cache"
	"cxcdn/internal/gh"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// GhFile handles /gh/{user}/{repo}(@{version})/{file}
func GhFile(ctx context.Context, c *app.RequestContext) {
	user := c.Param("user")
	repoInput := c.Param("repo")
	filePath := c.Param("file")

	repo, versionReq := gh.ParseRepoRef(repoInput)

	// Strip leading slash from file path (Hertz wildcard includes leading /)
	filePath = strings.TrimPrefix(filePath, "/")
	if filePath == "" {
		GhList(ctx, c)
		return
	}

	ref := versionReq
	if ref == "" {
		ref = "main"
	}

	data, contentType, err := gh.GetFileContent(user, repo, ref, filePath)
	if err != nil {
		c.String(consts.StatusNotFound, "File not found: %s (%v)", filePath, err)
		return
	}

	if contentType == "" || contentType == "text/plain" {
		contentType = mimeFromExt(filepath.Base(filePath))
	}

	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Data(consts.StatusOK, contentType, data)
}

// GhList handles /gh/{user}/{repo}(@{version})/
func GhList(ctx context.Context, c *app.RequestContext) {
	user := c.Param("user")
	repoInput := c.Param("repo")

	// Try response cache first
	cacheKey := "gh:page:" + user + "/" + repoInput
	if cached, found := cache.C.Get(cacheKey); found {
		resp := cached.(*cachedResponse)
		for k, v := range resp.Headers {
			c.Header(k, v)
		}
		c.Data(resp.Status, "text/html; charset=utf-8", resp.Body)
		return
	}

	repo, versionReq := gh.ParseRepoRef(repoInput)
	ref := versionReq
	if ref == "" {
		ref = "main"
	}

	prefix := ""

	files, err := gh.ListFiles(user, repo, ref, prefix)
	if err != nil {
		c.String(consts.StatusNotFound, "Error: %v", err)
		return
	}

	var fileNames []string
	for _, f := range files {
		if f.Type == "tree" {
			fileNames = append(fileNames, f.Path+"/")
		} else {
			fileNames = append(fileNames, f.Path)
		}
	}

	repoDisplayName := fmt.Sprintf("%s/%s", user, repo)
	if versionReq != "" {
		repoDisplayName = fmt.Sprintf("%s/%s@%s", user, repo, versionReq)
	}

	baseURL := fmt.Sprintf("/gh/%s/%s@%s/", user, repo, ref)

	// Build version selector from tags and branches
	var versionOptions []VersionOption
	tags, _ := gh.ListTags(user, repo)
	branches, _ := gh.ListBranches(user, repo)

	for _, b := range branches {
		versionOptions = append(versionOptions, VersionOption{
			Version: b,
			URL:     fmt.Sprintf("/gh/%s/%s@%s/", user, repo, b),
			Current: b == ref,
		})
	}
	for _, t := range tags {
		versionOptions = append(versionOptions, VersionOption{
			Version: t,
			URL:     fmt.Sprintf("/gh/%s/%s@%s/", user, repo, t),
			Current: t == ref,
		})
	}

	html := renderFileListHTML("github", repoDisplayName, fileNames, baseURL, versionOptions)

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

// GhListSubPath handles listing files under a sub-path for GitHub repos
func GhListSubPath(ctx context.Context, c *app.RequestContext) {
	user := c.Param("user")
	repoInput := c.Param("repo")
	subPath := c.Param("subpath")

	// Try response cache first
	cacheKey := "gh:page:" + user + "/" + repoInput + "/" + subPath
	if cached, found := cache.C.Get(cacheKey); found {
		resp := cached.(*cachedResponse)
		for k, v := range resp.Headers {
			c.Header(k, v)
		}
		c.Data(resp.Status, "text/html; charset=utf-8", resp.Body)
		return
	}

	repo, versionReq := gh.ParseRepoRef(repoInput)
	ref := versionReq
	if ref == "" {
		ref = "main"
	}

	prefix := strings.TrimPrefix(subPath, "/")
	prefix = strings.TrimSuffix(prefix, "/")

	files, err := gh.ListFiles(user, repo, ref, prefix)
	if err != nil {
		c.String(consts.StatusNotFound, "Error: %v", err)
		return
	}

	var fileNames []string
	for _, f := range files {
		name := f.Path
		if f.Type == "tree" {
			name += "/"
		}
		fileNames = append(fileNames, name)
	}

	repoDisplayName := fmt.Sprintf("%s/%s", user, repo)
	if versionReq != "" {
		repoDisplayName = fmt.Sprintf("%s/%s@%s", user, repo, versionReq)
	}

	baseURL := fmt.Sprintf("/gh/%s/%s@%s/%s/", user, repo, ref, prefix)

	// Build version selector from tags and branches
	var versionOptions []VersionOption
	tags, _ := gh.ListTags(user, repo)
	branches, _ := gh.ListBranches(user, repo)

	for _, b := range branches {
		versionOptions = append(versionOptions, VersionOption{
			Version: b,
			URL:     fmt.Sprintf("/gh/%s/%s@%s/%s/", user, repo, b, prefix),
			Current: b == ref,
		})
	}
	for _, t := range tags {
		versionOptions = append(versionOptions, VersionOption{
			Version: t,
			URL:     fmt.Sprintf("/gh/%s/%s@%s/%s/", user, repo, t, prefix),
			Current: t == ref,
		})
	}

	html := renderFileListHTML("github", repoDisplayName+" /"+prefix, fileNames, baseURL, versionOptions)

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
