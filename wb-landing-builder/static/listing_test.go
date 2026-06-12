package static

import (
	"strings"
	"testing"
)

func TestRenderListingHTML_colorsAndEntries(t *testing.T) {
	html := renderListingHTML("/assets/", "/assets", []dirEntry{
		{name: "images", isDir: true},
		{name: "app.js", isDir: false},
	})

	for _, want := range []string{
		"Index of /assets/",
		`class="dir" href="/assets/images/"`,
		`class="file" href="/assets/app.js"`,
		`class="parent" href="/"`,
		"Directories are blue, files are white",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected HTML to contain %q\n%s", want, html)
		}
	}
}

func TestRenderListingHTML_rootHasNoParentLink(t *testing.T) {
	html := renderListingHTML("/", "", []dirEntry{
		{name: "index.html", isDir: false},
	})

	if strings.Contains(html, `href="../"`) {
		t.Fatalf("root listing should not contain parent link:\n%s", html)
	}
	if !strings.Contains(html, `class="file" href="/index.html"`) {
		t.Fatalf("expected root file link:\n%s", html)
	}
}
