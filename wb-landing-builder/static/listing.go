package static

import (
	"fmt"
	"html"
	"io"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"
)

type dirEntry struct {
	name  string
	isDir bool
}

func writeDirectoryListing(w http.ResponseWriter, r *http.Request, urlPath string, dir fs.FS) error {
	entries, err := fs.ReadDir(dir, ".")
	if err != nil {
		return err
	}

	items := make([]dirEntry, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == "." {
			continue
		}
		items = append(items, dirEntry{name: name, isDir: entry.IsDir()})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].isDir != items[j].isDir {
			return items[i].isDir
		}
		return strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
	})

	displayPath := urlPath
	if displayPath == "" {
		displayPath = "/"
	} else if !strings.HasSuffix(displayPath, "/") {
		displayPath += "/"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return nil
	}

	_, err = io.WriteString(w, renderListingHTML(displayPath, urlPath, items))
	return err
}

func renderListingHTML(displayPath, urlPath string, items []dirEntry) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"UTF-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	fmt.Fprintf(&b, "<title>Index of %s</title>\n", html.EscapeString(displayPath))
	b.WriteString("<style>\n")
	b.WriteString("body { margin: 1.5rem; background: #1e1e1e; color: #d4d4d4; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }\n")
	b.WriteString("h1 { font-size: 1.1rem; font-weight: 600; color: #cccccc; }\n")
	b.WriteString("ul { list-style: none; padding: 0; margin: 0.75rem 0 0; line-height: 1.6; }\n")
	b.WriteString("a { text-decoration: none; }\n")
	b.WriteString("a:hover { text-decoration: underline; }\n")
	b.WriteString("a.dir { color: #569cd6; }\n")
	b.WriteString("a.file { color: #d4d4d4; }\n")
	b.WriteString("a.parent { color: #9cdcfe; }\n")
	b.WriteString(".hint { margin-top: 1rem; font-size: 0.85rem; color: #808080; }\n")
	b.WriteString("</style>\n</head>\n<body>\n")
	fmt.Fprintf(&b, "<h1>Index of %s</h1>\n<ul>\n", html.EscapeString(displayPath))

	if urlPath != "" && urlPath != "/" {
		parent := parentURLPath(urlPath)
		fmt.Fprintf(&b, "<li><a class=\"parent\" href=\"%s\">../</a></li>\n", html.EscapeString(parent))
	}

	for _, item := range items {
		href := joinURLPath(urlPath, item.name, item.isDir)
		class := "file"
		label := html.EscapeString(item.name)
		if item.isDir {
			class = "dir"
			label += "/"
		}
		fmt.Fprintf(&b, "<li><a class=\"%s\" href=\"%s\">%s</a></li>\n", class, html.EscapeString(href), label)
	}

	b.WriteString("</ul>\n")
	b.WriteString("<p class=\"hint\">Directories are blue, files are white — like <code>ls</code> in the terminal.</p>\n")
	b.WriteString("</body>\n</html>")
	return b.String()
}

func joinURLPath(base, name string, isDir bool) string {
	joined := path.Join(base, name)
	if isDir && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}
	return joined
}

func parentURLPath(urlPath string) string {
	trimmed := strings.TrimSuffix(urlPath, "/")
	if trimmed == "" || trimmed == "/" {
		return "/"
	}
	parent := path.Dir(trimmed)
	if parent == "." || parent == "/" {
		return "/"
	}
	return parent + "/"
}
