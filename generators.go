package s3gen

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SitemapGenerator generates a sitemap.xml in the Finalize phase.
// It collects all generated HTML pages and outputs them as a sitemap.
type SitemapGenerator struct {
	// BaseURL is the base URL for the site (e.g., "https://example.com")
	BaseURL string

	// OutputPath is the path to write the sitemap (default: "sitemap.xml")
	OutputPath string

	// ChangeFreq is the default change frequency for pages (default: "weekly")
	ChangeFreq string

	// Priority is the default priority for pages (default: 0.5)
	Priority float64

	// ExcludePatterns are glob patterns for paths to exclude from the sitemap
	ExcludePatterns []string

	// collected URLs during build
	urls []sitemapURL
}

type sitemapURL struct {
	Loc        string
	LastMod    time.Time
	ChangeFreq string
	Priority   float64
}

type sitemapXML struct {
	XMLName xml.Name        `xml:"urlset"`
	XMLNS   string          `xml:"xmlns,attr"`
	URLs    []sitemapURLXML `xml:"url"`
}

type sitemapURLXML struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod,omitempty"`
	ChangeFreq string  `xml:"changefreq,omitempty"`
	Priority   float64 `xml:"priority,omitempty"`
}

// Phase returns PhaseFinalize - sitemap generation happens after all content is generated.
func (g *SitemapGenerator) Phase() BuildPhase {
	return PhaseFinalize
}

// DependsOn returns patterns for HTML files - sitemap needs all pages generated first.
func (g *SitemapGenerator) DependsOn() []string {
	return []string{"**/*.html"}
}

// Produces returns the sitemap.xml pattern.
func (g *SitemapGenerator) Produces() []string {
	return []string{"sitemap.xml"}
}

// TargetsFor returns nil - SitemapGenerator uses hooks instead of per-resource targets.
func (g *SitemapGenerator) TargetsFor(site *Site, res *Resource) ([]*Resource, []*Resource) {
	return nil, nil
}

// Run is a no-op - actual work is done via hooks.
func (g *SitemapGenerator) Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error {
	return nil
}

// Register adds the sitemap generator to a site.
// This sets up hooks to collect URLs during the build and write the sitemap at the end.
func (g *SitemapGenerator) Register(site *Site) {
	// Set defaults
	if g.OutputPath == "" {
		g.OutputPath = "sitemap.xml"
	}
	if g.ChangeFreq == "" {
		g.ChangeFreq = "weekly"
	}
	if g.Priority == 0 {
		g.Priority = 0.5
	}

	// Initialize hooks if needed
	if site.Hooks == nil {
		site.Hooks = NewHookRegistry()
	}

	// Reset URLs at start of build
	site.Hooks.OnPhaseStart(PhaseDiscover, func(ctx *BuildContext) {
		g.urls = nil
	})

	// Collect URLs as resources are processed
	site.Hooks.OnResourceProcessed(func(ctx *BuildContext, res *Resource, targets []*Resource) {
		for _, target := range targets {
			if !strings.HasSuffix(target.FullPath, ".html") {
				continue
			}

			// Get relative path from output dir
			relPath, err := filepath.Rel(ctx.Site.OutputDir, target.FullPath)
			if err != nil {
				continue
			}

			// Check exclusions
			if g.shouldExclude(relPath) {
				continue
			}

			// Build URL path
			urlPath := "/" + strings.TrimSuffix(relPath, "index.html")
			urlPath = strings.TrimSuffix(urlPath, "/") + "/"
			if urlPath == "//" {
				urlPath = "/"
			}

			// Get last modified time
			var lastMod time.Time
			if info, err := os.Stat(target.FullPath); err == nil {
				lastMod = info.ModTime()
			}

			g.urls = append(g.urls, sitemapURL{
				Loc:        urlPath,
				LastMod:    lastMod,
				ChangeFreq: g.ChangeFreq,
				Priority:   g.Priority,
			})
		}
	})

	// Write sitemap at end of Finalize phase
	site.Hooks.OnPhaseEnd(PhaseFinalize, func(ctx *BuildContext) {
		if err := g.writeSitemap(ctx.Site.OutputDir); err != nil {
			ctx.AddError(fmt.Errorf("sitemap generation failed: %w", err))
		}
	})
}

func (g *SitemapGenerator) shouldExclude(path string) bool {
	for _, pattern := range g.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}

func (g *SitemapGenerator) writeSitemap(outputDir string) error {
	if len(g.urls) == 0 {
		return nil
	}

	sitemap := sitemapXML{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
	}

	for _, u := range g.urls {
		fullURL := strings.TrimSuffix(g.BaseURL, "/") + u.Loc
		entry := sitemapURLXML{
			Loc:        fullURL,
			ChangeFreq: u.ChangeFreq,
			Priority:   u.Priority,
		}
		if !u.LastMod.IsZero() {
			entry.LastMod = u.LastMod.Format("2006-01-02")
		}
		sitemap.URLs = append(sitemap.URLs, entry)
	}

	outPath := filepath.Join(outputDir, g.OutputPath)
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString(xml.Header)
	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	return enc.Encode(sitemap)
}

// RSSGenerator generates an RSS feed in the Finalize phase.
// It collects blog posts and outputs them as an RSS 2.0 feed.
type RSSGenerator struct {
	// Title is the feed title
	Title string

	// Description is the feed description
	Description string

	// BaseURL is the base URL for the site (e.g., "https://example.com")
	BaseURL string

	// FeedPath is the URL path for the feed (default: "/feed.xml")
	FeedPath string

	// OutputPath is the file path to write the feed (default: "feed.xml")
	OutputPath string

	// ContentPattern is a glob pattern to match content files for the feed
	// Default: "blog/**/*.html"
	ContentPattern string

	// MaxItems is the maximum number of items in the feed (default: 20)
	MaxItems int

	// collected items during build
	items []rssItem
}

type rssItem struct {
	Title       string
	Link        string
	Description string
	PubDate     time.Time
	GUID        string
}

type rssXML struct {
	XMLName xml.Name      `xml:"rss"`
	Version string        `xml:"version,attr"`
	Channel rssChannelXML `xml:"channel"`
}

type rssChannelXML struct {
	Title       string       `xml:"title"`
	Link        string       `xml:"link"`
	Description string       `xml:"description"`
	PubDate     string       `xml:"pubDate,omitempty"`
	Items       []rssItemXML `xml:"item"`
}

type rssItemXML struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description,omitempty"`
	PubDate     string `xml:"pubDate,omitempty"`
	GUID        string `xml:"guid,omitempty"`
}

// Phase returns PhaseFinalize - RSS generation happens after all content is generated.
func (g *RSSGenerator) Phase() BuildPhase {
	return PhaseFinalize
}

// DependsOn returns patterns for HTML files.
func (g *RSSGenerator) DependsOn() []string {
	return []string{"**/*.html"}
}

// Produces returns the feed.xml pattern.
func (g *RSSGenerator) Produces() []string {
	return []string{"feed.xml"}
}

// TargetsFor returns nil - RSSGenerator uses hooks instead of per-resource targets.
func (g *RSSGenerator) TargetsFor(site *Site, res *Resource) ([]*Resource, []*Resource) {
	return nil, nil
}

// Run is a no-op - actual work is done via hooks.
func (g *RSSGenerator) Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error {
	return nil
}

// Register adds the RSS generator to a site.
func (g *RSSGenerator) Register(site *Site) {
	// Set defaults
	if g.OutputPath == "" {
		g.OutputPath = "feed.xml"
	}
	if g.FeedPath == "" {
		g.FeedPath = "/feed.xml"
	}
	if g.ContentPattern == "" {
		g.ContentPattern = "blog/**/*.html"
	}
	if g.MaxItems == 0 {
		g.MaxItems = 20
	}

	// Initialize hooks if needed
	if site.Hooks == nil {
		site.Hooks = NewHookRegistry()
	}

	// Reset items at start of build
	site.Hooks.OnPhaseStart(PhaseDiscover, func(ctx *BuildContext) {
		g.items = nil
	})

	// Collect items as resources are processed
	site.Hooks.OnResourceProcessed(func(ctx *BuildContext, res *Resource, targets []*Resource) {
		for _, target := range targets {
			if !strings.HasSuffix(target.FullPath, ".html") {
				continue
			}

			// Get relative path from output dir
			relPath, err := filepath.Rel(ctx.Site.OutputDir, target.FullPath)
			if err != nil {
				continue
			}

			// Check if matches content pattern
			if matched, _ := filepath.Match(g.ContentPattern, relPath); !matched {
				// Try with ** expansion
				if !strings.Contains(relPath, "blog/") {
					continue
				}
			}

			// Skip index pages (listing pages)
			if relPath == "blog/index.html" {
				continue
			}

			// Get metadata from source resource
			var title, description string
			var pubDate time.Time

			if res != nil && res.FrontMatter() != nil {
				fm := res.FrontMatter().Data
				if t, ok := fm["title"].(string); ok {
					title = t
				}
				if d, ok := fm["description"].(string); ok {
					description = d
				}
				if d, ok := fm["date"].(string); ok {
					pubDate, _ = time.Parse("2006-01-02", d)
				}
				if d, ok := fm["date"].(time.Time); ok {
					pubDate = d
				}
			}

			if title == "" {
				continue // Skip items without title
			}

			// Build URL path
			urlPath := "/" + strings.TrimSuffix(relPath, "index.html")
			urlPath = strings.TrimSuffix(urlPath, "/") + "/"

			g.items = append(g.items, rssItem{
				Title:       title,
				Link:        urlPath,
				Description: description,
				PubDate:     pubDate,
				GUID:        urlPath,
			})
		}
	})

	// Write feed at end of Finalize phase
	site.Hooks.OnPhaseEnd(PhaseFinalize, func(ctx *BuildContext) {
		if err := g.writeFeed(ctx.Site.OutputDir); err != nil {
			ctx.AddError(fmt.Errorf("RSS generation failed: %w", err))
		}
	})
}

func (g *RSSGenerator) writeFeed(outputDir string) error {
	if len(g.items) == 0 {
		return nil
	}

	// Sort by date (newest first) and limit
	// Simple bubble sort for small lists
	for i := 0; i < len(g.items)-1; i++ {
		for j := i + 1; j < len(g.items); j++ {
			if g.items[j].PubDate.After(g.items[i].PubDate) {
				g.items[i], g.items[j] = g.items[j], g.items[i]
			}
		}
	}

	items := g.items
	if len(items) > g.MaxItems {
		items = items[:g.MaxItems]
	}

	feed := rssXML{
		Version: "2.0",
		Channel: rssChannelXML{
			Title:       g.Title,
			Link:        g.BaseURL,
			Description: g.Description,
		},
	}

	if len(items) > 0 && !items[0].PubDate.IsZero() {
		feed.Channel.PubDate = items[0].PubDate.Format(time.RFC1123Z)
	}

	for _, item := range items {
		fullURL := strings.TrimSuffix(g.BaseURL, "/") + item.Link
		entry := rssItemXML{
			Title:       item.Title,
			Link:        fullURL,
			Description: item.Description,
			GUID:        fullURL,
		}
		if !item.PubDate.IsZero() {
			entry.PubDate = item.PubDate.Format(time.RFC1123Z)
		}
		feed.Channel.Items = append(feed.Channel.Items, entry)
	}

	outPath := filepath.Join(outputDir, g.OutputPath)
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString(xml.Header)
	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	return enc.Encode(feed)
}
