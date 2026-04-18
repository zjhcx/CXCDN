package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cxcdn/internal/cache"
	"cxcdn/internal/pool"

	"github.com/cloudwego/hertz/pkg/protocol"
)

type GitHubRelease struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

type GitHubTag struct {
	Name string `json:"name"`
}

type GitHubTreeItem struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"` // "blob" or "tree"
	Sha  string `json:"sha"`
	Size int64  `json:"size,omitempty"`
}

type GitHubTree struct {
	Sha       string           `json:"sha"`
	Tree      []GitHubTreeItem `json:"tree"`
	Truncated bool             `json:"truncated"`
}

// ParseRepoRef parses "repo@version" into (repo, version)
func ParseRepoRef(input string) (string, string) {
	parts := strings.SplitN(input, "@", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// ResolveRef resolves a git ref (branch, tag, commit) to a commit SHA
func ResolveRef(owner, repo, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}

	cacheKey := fmt.Sprintf("gh:ref:%s/%s@%s", owner, repo, ref)
	if cached, found := cache.C.Get(cacheKey); found {
		return cached.(string), nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repo, ref)
	c := pool.Get("api.github.com")

	req := protocol.AcquireRequest()
	defer protocol.ReleaseRequest(req)
	req.SetMethod("GET")
	req.SetRequestURI(url)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp := protocol.AcquireResponse()
	defer protocol.ReleaseResponse(resp)

	if err := c.Do(context.Background(), req, resp); err != nil {
		return "", fmt.Errorf("failed to resolve ref %s: %w", ref, err)
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("github returned status %d for ref %s", resp.StatusCode(), ref)
	}

	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("failed to parse github ref response: %w", err)
	}

	cache.C.Set(cacheKey, result.SHA, cache.DefaultExpiration)
	return result.SHA, nil
}

// GetTree fetches the git tree for a repo at a given ref
func GetTree(owner, repo, ref string) (*GitHubTree, error) {
	sha, err := ResolveRef(owner, repo, ref)
	if err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("gh:tree:%s/%s:%s", owner, repo, sha)
	if cached, found := cache.C.Get(cacheKey); found {
		return cached.(*GitHubTree), nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", owner, repo, sha)
	c := pool.Get("api.github.com")

	req := protocol.AcquireRequest()
	defer protocol.ReleaseRequest(req)
	req.SetMethod("GET")
	req.SetRequestURI(url)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp := protocol.AcquireResponse()
	defer protocol.ReleaseResponse(resp)

	if err := c.Do(context.Background(), req, resp); err != nil {
		return nil, fmt.Errorf("failed to fetch tree: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("github returned status %d for tree", resp.StatusCode())
	}

	var tree GitHubTree
	if err := json.Unmarshal(resp.Body(), &tree); err != nil {
		return nil, fmt.Errorf("failed to parse github tree: %w", err)
	}

	cache.C.Set(cacheKey, &tree, cache.DefaultExpiration)
	return &tree, nil
}

// GetFileContent fetches a single file's content from GitHub
func GetFileContent(owner, repo, ref, filePath string) ([]byte, string, error) {
	if ref == "" {
		ref = "HEAD"
	}

	cacheKey := fmt.Sprintf("gh:file:%s/%s@%s:%s", owner, repo, ref, filePath)
	if cached, found := cache.C.Get(cacheKey); found {
		pair := cached.([2]string)
		return []byte(pair[0]), pair[1], nil
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, filePath)
	c := pool.Get("raw.githubusercontent.com")

	req := protocol.AcquireRequest()
	defer protocol.ReleaseRequest(req)
	req.SetMethod("GET")
	req.SetRequestURI(url)

	resp := protocol.AcquireResponse()
	defer protocol.ReleaseResponse(resp)

	if err := c.Do(context.Background(), req, resp); err != nil {
		return nil, "", fmt.Errorf("failed to fetch file: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, "", fmt.Errorf("github returned status %d for file %s", resp.StatusCode(), filePath)
	}

	data := resp.Body()
	contentType := string(resp.Header.Peek("Content-Type"))
	pair := [2]string{string(data), contentType}
	cache.C.Set(cacheKey, pair, cache.DefaultExpiration)
	return data, contentType, nil
}

// ListFiles returns files under a given path prefix
func ListFiles(owner, repo, ref, prefix string) ([]GitHubTreeItem, error) {
	tree, err := GetTree(owner, repo, ref)
	if err != nil {
		return nil, err
	}

	var files []GitHubTreeItem
	for _, item := range tree.Tree {
		if prefix == "" {
			if !strings.Contains(item.Path, "/") {
				files = append(files, item)
			}
		} else {
			prefixWithSlash := prefix + "/"
			if strings.HasPrefix(item.Path, prefixWithSlash) {
				rel := strings.TrimPrefix(item.Path, prefixWithSlash)
				if !strings.Contains(rel, "/") && rel != "" {
					files = append(files, item)
				}
			}
		}
	}

	return files, nil
}

// ListTags returns tags for a GitHub repo
func ListTags(owner, repo string) ([]string, error) {
	cacheKey := fmt.Sprintf("gh:tags:%s/%s", owner, repo)
	if cached, found := cache.C.Get(cacheKey); found {
		return cached.([]string), nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags?per_page=100", owner, repo)
	c := pool.Get("api.github.com")

	req := protocol.AcquireRequest()
	defer protocol.ReleaseRequest(req)
	req.SetMethod("GET")
	req.SetRequestURI(url)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp := protocol.AcquireResponse()
	defer protocol.ReleaseResponse(resp)

	if err := c.Do(context.Background(), req, resp); err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("github returned status %d for tags", resp.StatusCode())
	}

	var tags []GitHubTag
	if err := json.Unmarshal(resp.Body(), &tags); err != nil {
		return nil, fmt.Errorf("failed to parse github tags: %w", err)
	}

	var names []string
	for _, t := range tags {
		names = append(names, t.Name)
	}

	cache.C.Set(cacheKey, names, cache.DefaultExpiration)
	return names, nil
}

// ListBranches returns branches for a GitHub repo
func ListBranches(owner, repo string) ([]string, error) {
	cacheKey := fmt.Sprintf("gh:branches:%s/%s", owner, repo)
	if cached, found := cache.C.Get(cacheKey); found {
		return cached.([]string), nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches?per_page=100", owner, repo)
	c := pool.Get("api.github.com")

	req := protocol.AcquireRequest()
	defer protocol.ReleaseRequest(req)
	req.SetMethod("GET")
	req.SetRequestURI(url)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp := protocol.AcquireResponse()
	defer protocol.ReleaseResponse(resp)

	if err := c.Do(context.Background(), req, resp); err != nil {
		return nil, fmt.Errorf("failed to fetch branches: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("github returned status %d for branches", resp.StatusCode())
	}

	var branches []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp.Body(), &branches); err != nil {
		return nil, fmt.Errorf("failed to parse github branches: %w", err)
	}

	var names []string
	for _, b := range branches {
		names = append(names, b.Name)
	}

	cache.C.Set(cacheKey, names, cache.DefaultExpiration)
	return names, nil
}
