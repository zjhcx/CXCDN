package storage

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cxcdn/internal/cache"
	"cxcdn/internal/pool"

	"github.com/cloudwego/hertz/pkg/protocol"
)

const CacheDir = ".cache/npm"

var fetchSemaphore = make(chan struct{}, 50)

// Per-package locks to avoid global mutex contention
var (
	pkgMu    sync.Mutex
	pkgLocks = make(map[string]*sync.Mutex)
)

func getPkgLock(dirName string) *sync.Mutex {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	if m, ok := pkgLocks[dirName]; ok {
		return m
	}
	m := &sync.Mutex{}
	pkgLocks[dirName] = m
	return m
}

// FetchAndExtractTarball downloads an npm tarball and extracts it to local cache
func FetchAndExtractTarball(name, version, tarballURL string) (string, error) {
	dirName := fmt.Sprintf("%s@%s", sanitizeName(name), version)
	localDir := filepath.Join(CacheDir, dirName)

	// Fast path: already extracted on disk, no lock needed
	if _, err := os.Stat(localDir); err == nil {
		return localDir, nil
	}

	// Acquire semaphore to limit concurrent downloads
	fetchSemaphore <- struct{}{}
	defer func() { <-fetchSemaphore }()

	// Slow path: need to extract, use per-package lock
	mu := getPkgLock(dirName)
	mu.Lock()
	defer mu.Unlock()

	// Double-check after acquiring lock
	if _, err := os.Stat(localDir); err == nil {
		return localDir, nil
	}

	// Use non-blocking Hertz client for tarball download
	host := extractHost(tarballURL)
	c := pool.Get(host)

	req := protocol.AcquireRequest()
	defer protocol.ReleaseRequest(req)
	req.SetMethod("GET")
	req.SetRequestURI(tarballURL)

	resp := protocol.AcquireResponse()
	defer protocol.ReleaseResponse(resp)

	if err := c.Do(context.Background(), req, resp); err != nil {
		return "", fmt.Errorf("failed to download tarball: %w", err)
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("tarball download returned status %d", resp.StatusCode())
	}

	tmpFile, err := os.CreateTemp("", "npm-tarball-*.tgz")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(resp.Body()); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to save tarball: %w", err)
	}
	tmpFile.Close()

	if err := os.MkdirAll(localDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create extraction dir: %w", err)
	}

	if err := extractTarGz(tmpFile.Name(), localDir); err != nil {
		os.RemoveAll(localDir)
		return "", fmt.Errorf("failed to extract tarball: %w", err)
	}

	return localDir, nil
}

func extractTarGz(tarballPath, dest string) error {
	f, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cleanPath := hdr.Name
		if strings.HasPrefix(cleanPath, "package/") {
			cleanPath = strings.TrimPrefix(cleanPath, "package/")
		}
		if cleanPath == "package" {
			continue
		}

		target := filepath.Join(dest, cleanPath)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				continue
			}
		}
	}

	return nil
}

// GetFileFromPackage returns the content of a file from an extracted npm package
func GetFileFromPackage(name, version, tarballURL, filePath string) ([]byte, error) {
	cacheKey := fmt.Sprintf("npm:file:%s@%s:%s", sanitizeName(name), version, filePath)
	if cached, found := cache.C.Get(cacheKey); found {
		return cached.([]byte), nil
	}

	localDir, err := FetchAndExtractTarball(name, version, tarballURL)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(localDir, filePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	cache.C.Set(cacheKey, data, cache.DefaultExpiration)
	return data, nil
}

// ListPackageFiles lists files in an extracted npm package
func ListPackageFiles(name, version, tarballURL string) ([]string, error) {
	cacheKey := fmt.Sprintf("npm:files:%s@%s", sanitizeName(name), version)
	if cached, found := cache.C.Get(cacheKey); found {
		return cached.([]string), nil
	}

	localDir, err := FetchAndExtractTarball(name, version, tarballURL)
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	cache.C.Set(cacheKey, files, cache.DefaultExpiration)
	return files, err
}

func sanitizeName(name string) string {
	return strings.ReplaceAll(name, "/", "_")
}

// extractHost extracts the hostname from a URL
func extractHost(rawURL string) string {
	host := rawURL
	// Remove scheme
	if idx := strings.Index(host, "://"); idx != -1 {
		host = host[idx+3:]
	}
	// Remove path
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}
	// Remove port
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return host
}
