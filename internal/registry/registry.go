package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"cxcdn/internal/cache"
	"cxcdn/internal/pool"

	"github.com/cloudwego/hertz/pkg/protocol"
)

type NpmPackage struct {
	Name     string                `json:"name"`
	Versions map[string]NpmVersion `json:"versions"`
	DistTags map[string]string     `json:"dist-tags"`
}

type NpmVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Dist    struct {
		Tarball string `json:"tarball"`
	} `json:"dist"`
}

var tarballClient = pool.GetTarballClient()

func ResolveNpmPackage(name string) (*NpmPackage, error) {
	cacheKey := "npm:pkg:" + name
	if cached, found := cache.C.Get(cacheKey); found {
		return cached.(*NpmPackage), nil
	}

	url := "https://registry.npmjs.org/" + name
	c := pool.Get("registry.npmjs.org")

	req := protocol.AcquireRequest()
	defer protocol.ReleaseRequest(req)
	req.SetMethod("GET")
	req.SetRequestURI(url)

	resp := protocol.AcquireResponse()
	defer protocol.ReleaseResponse(resp)

	if err := c.Do(context.Background(), req, resp); err != nil {
		return nil, fmt.Errorf("failed to fetch npm package %s: %w", name, err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("npm registry returned status %d for %s", resp.StatusCode(), name)
	}

	var pkg NpmPackage
	if err := json.Unmarshal(resp.Body(), &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse npm response: %w", err)
	}

	cache.C.Set(cacheKey, &pkg, cache.DefaultExpiration)
	return &pkg, nil
}

func ResolveNpmVersion(name, version string) (string, string, error) {
	pkg, err := ResolveNpmPackage(name)
	if err != nil {
		return "", "", err
	}

	if version == "" {
		version = "latest"
	}

	if tag, ok := pkg.DistTags[version]; ok {
		version = tag
	}

	v, ok := pkg.Versions[version]
	if !ok {
		return "", "", fmt.Errorf("version %s not found for package %s", version, name)
	}

	return version, v.Dist.Tarball, nil
}

// GetNpmVersions returns all available versions for an npm package
func GetNpmVersions(name string) ([]string, string, error) {
	pkg, err := ResolveNpmPackage(name)
	if err != nil {
		return nil, "", err
	}

	versions := make([]string, 0, len(pkg.Versions))
	for v := range pkg.Versions {
		versions = append(versions, v)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))

	latest := ""
	if tag, ok := pkg.DistTags["latest"]; ok {
		latest = tag
	}

	return versions, latest, nil
}

// ParsePackageName parses "package@version" or "package" into (name, version)
func ParsePackageName(input string) (string, string) {
	if strings.HasPrefix(input, "@") {
		idx := strings.Index(input[1:], "@")
		if idx == -1 {
			return input, ""
		}
		return input[:idx+1], input[idx+2:]
	}

	parts := strings.SplitN(input, "@", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}
