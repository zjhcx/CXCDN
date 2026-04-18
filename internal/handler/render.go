package handler

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/cloudwego/hertz/pkg/app"
)

var (
	indexTmpl     *template.Template
	indexTmplOnce sync.Once

	fileListTmpl     *template.Template
	fileListTmplOnce sync.Once
)

type FileEntry struct {
	Name    string
	IsDir   bool
	Path    string
	Size    string
	ModTime string
}

type VersionOption struct {
	Version string
	URL     string
	Current bool
}

func renderFileList(ctx context.Context, c *app.RequestContext, source, packageName string, files []string, baseURL string, versions []VersionOption) {
	html := renderFileListHTML(source, packageName, files, baseURL, versions)
	c.Data(200, "text/html; charset=utf-8", html)
}

func renderFileListHTML(source, packageName string, files []string, baseURL string, versions []VersionOption) []byte {
	var entries []FileEntry

	sort.Slice(files, func(i, j int) bool {
		iDir := strings.HasSuffix(files[i], "/")
		jDir := strings.HasSuffix(files[j], "/")
		if iDir != jDir {
			return iDir
		}
		return files[i] < files[j]
	})

	for _, f := range files {
		isDir := strings.HasSuffix(f, "/")
		name := strings.TrimSuffix(f, "/")
		displayName := filepath.Base(name)
		if displayName == "" {
			displayName = name
		}

		path := f
		if !strings.HasPrefix(path, "/") {
			path = baseURL + f
		}

		entries = append(entries, FileEntry{
			Name:  displayName,
			IsDir: isDir,
			Path:  path,
		})
	}

	data := map[string]interface{}{
		"Source":      source,
		"PackageName": packageName,
		"BaseURL":     baseURL,
		"Entries":     entries,
		"Versions":    versions,
	}

	fileListTmplOnce.Do(func() {
		fileListTmpl = template.Must(template.New("filelist").Parse(fileListTemplate))
	})
	var buf bytes.Buffer
	fileListTmpl.Execute(&buf, data)
	return buf.Bytes()
}

const fileListTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.PackageName}} - CXCDN</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #0d1117;
            color: #c9d1d9;
            line-height: 1.6;
        }
        .container {
            max-width: 960px;
            margin: 0 auto;
            padding: 40px 20px;
        }
        .header {
            margin-bottom: 32px;
            padding-bottom: 16px;
            border-bottom: 1px solid #21262d;
        }
        .header h1 {
            font-size: 24px;
            font-weight: 600;
            color: #58a6ff;
            margin-bottom: 4px;
        }
        .header .meta {
            font-size: 14px;
            color: #8b949e;
        }
        .source-badge {
            display: inline-block;
            padding: 2px 8px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: 600;
            text-transform: uppercase;
            margin-right: 8px;
        }
        .source-badge.npm { background: #cb3837; color: #fff; }
        .source-badge.github { background: #238636; color: #fff; }
        .version-selector {
            margin-bottom: 24px;
            display: flex;
            align-items: center;
            gap: 12px;
        }
        .version-selector label {
            font-size: 14px;
            color: #8b949e;
            font-weight: 500;
        }
        .version-selector select {
            background: #161b22;
            color: #c9d1d9;
            border: 1px solid #30363d;
            border-radius: 6px;
            padding: 6px 32px 6px 12px;
            font-size: 14px;
            font-family: 'SF Mono', 'Fira Code', Menlo, monospace;
            cursor: pointer;
            appearance: none;
            -webkit-appearance: none;
            background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%238b949e' d='M3 4.5L6 8l3-3.5H3z'/%3E%3C/svg%3E");
            background-repeat: no-repeat;
            background-position: right 10px center;
            max-width: 300px;
        }
        .version-selector select:hover {
            border-color: #58a6ff;
        }
        .version-selector select:focus {
            outline: none;
            border-color: #58a6ff;
            box-shadow: 0 0 0 3px rgba(88, 166, 255, 0.3);
        }
        .file-list {
            border: 1px solid #21262d;
            border-radius: 6px;
            overflow: hidden;
        }
        .file-entry {
            display: flex;
            align-items: center;
            padding: 8px 16px;
            border-bottom: 1px solid #21262d;
            text-decoration: none;
            color: #c9d1d9;
            transition: background 0.15s;
        }
        .file-entry:last-child { border-bottom: none; }
        .file-entry:hover { background: #161b22; }
        .file-entry .icon {
            margin-right: 12px;
            width: 16px;
            text-align: center;
            font-size: 14px;
        }
        .file-entry .icon.dir { color: #54aeff; }
        .file-entry .icon.file { color: #8b949e; }
        .file-entry .name { flex: 1; }
        .file-entry .name a {
            color: #58a6ff;
            text-decoration: none;
        }
        .file-entry .name a:hover { text-decoration: underline; }
        .file-entry.dir .name a { color: #54aeff; font-weight: 500; }
        .empty {
            text-align: center;
            padding: 40px;
            color: #8b949e;
        }
        .footer {
            margin-top: 40px;
            padding-top: 16px;
            border-top: 1px solid #21262d;
            text-align: center;
            color: #8b949e;
            font-size: 13px;
        }
        .footer a { color: #58a6ff; text-decoration: none; }
        .footer a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>
                <span class="source-badge {{.Source}}">{{.Source}}</span>
                {{.PackageName}}
            </h1>
            <div class="meta">File listing</div>
        </div>

        {{if .Versions}}
        <div class="version-selector">
            <label>Version:</label>
            <select onchange="if(this.value)location.href=this.value">
                {{range .Versions}}
                <option value="{{.URL}}" {{if .Current}}selected{{end}}>{{.Version}}</option>
                {{end}}
            </select>
        </div>
        {{end}}

        <div class="file-list">
            {{if .Entries}}
                {{range .Entries}}
                <div class="file-entry {{if .IsDir}}dir{{else}}file{{end}}">
                    <span class="icon {{if .IsDir}}dir{{else}}file{{end}}">
                        {{if .IsDir}}&#128193;{{else}}&#128196;{{end}}
                    </span>
                    <span class="name"><a href="{{.Path}}">{{.Name}}{{if .IsDir}}/{{end}}</a></span>
                </div>
                {{end}}
            {{else}}
                <div class="empty">No files found</div>
            {{end}}
        </div>

        <div class="footer">
            Powered by <a href="/">CXCDN</a> &mdash; A jsDelivr-like CDN service
        </div>
    </div>
</body>
</html>
`

func RenderIndexHandler(ctx context.Context, c *app.RequestContext) {
	indexTmplOnce.Do(func() {
		indexTmpl = template.Must(template.New("index").Parse(indexTemplate))
	})
	var buf bytes.Buffer
	indexTmpl.Execute(&buf, nil)
	c.Data(200, "text/html; charset=utf-8", buf.Bytes())
}

const indexTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CXCDN - A jsDelivr-like CDN</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #0d1117;
            color: #c9d1d9;
            line-height: 1.6;
        }
        .hero {
            text-align: center;
            padding: 80px 20px;
        }
        .hero h1 {
            font-size: 48px;
            font-weight: 700;
            background: linear-gradient(135deg, #ff7eb3, #ff758c, #ff7eb3, #7eb3ff);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            margin-bottom: 16px;
        }
        .hero p {
            font-size: 18px;
            color: #8b949e;
            max-width: 600px;
            margin: 0 auto 40px;
        }
        .examples {
            max-width: 800px;
            margin: 0 auto;
            padding: 0 20px 60px;
        }
        .example-card {
            background: #161b22;
            border: 1px solid #21262d;
            border-radius: 8px;
            padding: 24px;
            margin-bottom: 16px;
        }
        .example-card h3 {
            color: #58a6ff;
            margin-bottom: 12px;
            font-size: 16px;
        }
        .example-card code {
            display: block;
            background: #0d1117;
            border: 1px solid #21262d;
            border-radius: 4px;
            padding: 12px 16px;
            font-family: 'SF Mono', 'Fira Code', 'Fira Mono', Menlo, monospace;
            font-size: 14px;
            color: #79c0ff;
            overflow-x: auto;
            margin-bottom: 8px;
        }
        .example-card .desc {
            font-size: 13px;
            color: #8b949e;
        }
        .example-card a {
            color: #58a6ff;
            text-decoration: none;
        }
        .example-card a:hover { text-decoration: underline; }
        .footer {
            text-align: center;
            padding: 40px 20px;
            color: #8b949e;
            font-size: 13px;
            border-top: 1px solid #21262d;
        }
        .badge {
            display: inline-block;
            padding: 2px 8px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: 600;
            margin-right: 4px;
        }
        .badge.npm { background: #cb3837; color: #fff; }
        .badge.gh { background: #238636; color: #fff; }
    </style>
</head>
<body>
    <div class="hero">
        <h1>CXCDN</h1>
        <p>A free, fast CDN for npm and GitHub, inspired by jsDelivr. Serving files from npm packages and GitHub repositories with caching and global delivery.</p>
    </div>

    <div class="examples">
        <div class="example-card">
            <h3><span class="badge npm">NPM</span> Package File</h3>
            <code>/npm/{package}@{version}/{file}</code>
            <code>/npm/vue@3.3.4/dist/vue.global.js</code>
            <div class="desc">
                Serve a specific file from an npm package. 
                <a href="/npm/vue@3.3.4/">Browse vue@3.3.4 files</a>
            </div>
        </div>

        <div class="example-card">
            <h3><span class="badge npm">NPM</span> File Listing</h3>
            <code>/npm/{package}@{version}/</code>
            <code>/npm/lodash@4.17.21/</code>
            <div class="desc">
                List all files in an npm package. Omit version for latest.
            </div>
        </div>

        <div class="example-card">
            <h3><span class="badge gh">GitHub</span> Repository File</h3>
            <code>/gh/{user}/{repo}@{version}/{file}</code>
            <code>/gh/vuejs/core@v3.3.4/packages/vue/dist/vue.global.js</code>
            <div class="desc">
                Serve a file from a GitHub repository. Version can be a branch, tag, or commit SHA.
            </div>
        </div>

        <div class="example-card">
            <h3><span class="badge gh">GitHub</span> File Listing</h3>
            <code>/gh/{user}/{repo}@{version}/</code>
            <code>/gh/facebook/react@main/</code>
            <div class="desc">
                List files in a GitHub repository. Omit version to use the default branch.
            </div>
        </div>

        <div class="example-card">
            <h3>Scoped Packages</h3>
            <code>/npm/@vue/cli@5.0.8/package.json</code>
            <div class="desc">
                Scoped npm packages are supported with the @ prefix.
            </div>
        </div>
    </div>

    <div class="footer">
        Powered by CXCDN &mdash; Built with Go + Hertz
    </div>
</body>
</html>
`

func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}
