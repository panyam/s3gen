
# Generators and Hooks

Generators are rules that produce site-wide artifacts like sitemaps and RSS feeds. They use hooks to collect data during the build and write output in the Finalize phase.

## Built-in Generators

### SitemapGenerator

Generates a `sitemap.xml` file containing all HTML pages.

```go
sitemapGen := &s3.SitemapGenerator{
    // Base URL for the site (required)
    BaseURL: "https://example.com",

    // Output file path (default: "sitemap.xml")
    OutputPath: "sitemap.xml",

    // Default change frequency (default: "weekly")
    ChangeFreq: "weekly",

    // Default priority (default: 0.5)
    Priority: 0.5,

    // Glob patterns for paths to exclude
    ExcludePatterns: []string{"404.html", "test/**"},
}

// Register with site
sitemapGen.Register(&Site)
```

**Output:**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/</loc>
    <lastmod>2025-11-29</lastmod>
    <changefreq>weekly</changefreq>
    <priority>0.5</priority>
  </url>
  <url>
    <loc>https://example.com/blog/my-post/</loc>
    <lastmod>2025-11-28</lastmod>
    <changefreq>weekly</changefreq>
    <priority>0.5</priority>
  </url>
</urlset>
```

### RSSGenerator

Generates an RSS 2.0 feed of your blog posts.

```go
rssGen := &s3.RSSGenerator{
    // Feed title (required)
    Title: "My Blog",

    // Feed description
    Description: "Latest posts from my blog",

    // Base URL for the site (required)
    BaseURL: "https://example.com",

    // URL path for the feed (default: "/feed.xml")
    FeedPath: "/feed.xml",

    // Output file path (default: "feed.xml")
    OutputPath: "feed.xml",

    // Glob pattern for content to include (default: "blog/**/*.html")
    ContentPattern: "blog/**/*.html",

    // Maximum items in feed (default: 20)
    MaxItems: 20,
}

// Register with site
rssGen.Register(&Site)
```

**Output:**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>My Blog</title>
    <link>https://example.com</link>
    <description>Latest posts from my blog</description>
    <pubDate>Fri, 29 Nov 2025 12:00:00 +0000</pubDate>
    <item>
      <title>My Latest Post</title>
      <link>https://example.com/blog/my-post/</link>
      <description>A great post about something</description>
      <pubDate>Thu, 28 Nov 2025 12:00:00 +0000</pubDate>
      <guid>https://example.com/blog/my-post/</guid>
    </item>
  </channel>
</rss>
```

## Using Generators

```go
package main

import (
    s3 "github.com/panyam/s3gen"
)

var Site = s3.Site{
    ContentRoot: "./content",
    OutputDir:   "./public",
    // ... other config
}

var sitemapGen = &s3.SitemapGenerator{
    BaseURL: "https://example.com",
}

var rssGen = &s3.RSSGenerator{
    Title:       "My Blog",
    Description: "Latest posts",
    BaseURL:     "https://example.com",
}

func main() {
    // Register generators BEFORE building
    sitemapGen.Register(&Site)
    rssGen.Register(&Site)

    // Now build
    Site.Rebuild(nil)
}
```

## The Hook System

Hooks allow you to observe build events without implementing a full rule. They're the foundation for generators.

### HookRegistry

```go
type HookRegistry struct {
    onPhaseStart      map[BuildPhase][]func(*BuildContext)
    onPhaseEnd        map[BuildPhase][]func(*BuildContext)
    onResourceProcess []func(*BuildContext, *Resource, []*Resource)
}
```

### Hook Types

#### OnPhaseStart

Called when a build phase begins.

```go
site.Hooks.OnPhaseStart(s3.PhaseDiscover, func(ctx *s3.BuildContext) {
    log.Println("Starting discovery...")
})
```

#### OnPhaseEnd

Called when a build phase ends.

```go
site.Hooks.OnPhaseEnd(s3.PhaseFinalize, func(ctx *s3.BuildContext) {
    log.Printf("Build complete: %d targets generated", len(ctx.GeneratedTargets))
})
```

#### OnResourceProcessed

Called after each resource is processed.

```go
site.Hooks.OnResourceProcessed(func(ctx *s3.BuildContext, res *s3.Resource, targets []*s3.Resource) {
    for _, target := range targets {
        log.Printf("Generated: %s", target.FullPath)
    }
})
```

### Initializing Hooks

```go
// Hooks are lazily initialized, but you can ensure they exist:
if site.Hooks == nil {
    site.Hooks = s3.NewHookRegistry()
}

// Then register your callbacks
site.Hooks.OnPhaseEnd(s3.PhaseFinalize, myCallback)
```

## Creating Custom Generators

### Basic Pattern

```go
type SearchIndexGenerator struct {
    OutputPath string
    entries    []SearchEntry
}

type SearchEntry struct {
    Title   string
    URL     string
    Content string
}

func (g *SearchIndexGenerator) Register(site *s3.Site) {
    // Initialize hooks
    if site.Hooks == nil {
        site.Hooks = s3.NewHookRegistry()
    }

    // Reset at start of build
    site.Hooks.OnPhaseStart(s3.PhaseDiscover, func(ctx *s3.BuildContext) {
        g.entries = nil
    })

    // Collect data during build
    site.Hooks.OnResourceProcessed(func(ctx *s3.BuildContext, res *s3.Resource, targets []*s3.Resource) {
        // Only process content files
        if res.FrontMatter() == nil {
            return
        }

        fm := res.FrontMatter().Data
        title, _ := fm["title"].(string)
        if title == "" {
            return
        }

        for _, target := range targets {
            if !strings.HasSuffix(target.FullPath, ".html") {
                continue
            }

            relPath, _ := filepath.Rel(ctx.Site.OutputDir, target.FullPath)
            url := "/" + strings.TrimSuffix(relPath, "index.html")

            g.entries = append(g.entries, SearchEntry{
                Title:   title,
                URL:     url,
                Content: extractText(res),
            })
        }
    })

    // Write output at end
    site.Hooks.OnPhaseEnd(s3.PhaseFinalize, func(ctx *s3.BuildContext) {
        g.writeIndex(ctx.Site.OutputDir)
    })
}

func (g *SearchIndexGenerator) writeIndex(outputDir string) {
    data, _ := json.MarshalIndent(g.entries, "", "  ")
    outPath := filepath.Join(outputDir, g.OutputPath)
    os.WriteFile(outPath, data, 0644)
}
```

### Implementing PhaseRule Interface

Generators can also implement `PhaseRule` for better integration:

```go
func (g *SearchIndexGenerator) Phase() s3.BuildPhase {
    return s3.PhaseFinalize
}

func (g *SearchIndexGenerator) DependsOn() []string {
    return []string{"**/*.html"}
}

func (g *SearchIndexGenerator) Produces() []string {
    return []string{"search-index.json"}
}

func (g *SearchIndexGenerator) TargetsFor(site *s3.Site, res *s3.Resource) ([]*s3.Resource, []*s3.Resource) {
    return nil, nil  // Uses hooks instead
}

func (g *SearchIndexGenerator) Run(site *s3.Site, inputs []*s3.Resource, targets []*s3.Resource, funcs map[string]any) error {
    return nil  // Uses hooks instead
}
```

## Advanced Hook Patterns

### Aggregating Data

```go
type TagCloudGenerator struct {
    tagCounts map[string]int
}

func (g *TagCloudGenerator) Register(site *s3.Site) {
    if site.Hooks == nil {
        site.Hooks = s3.NewHookRegistry()
    }

    // Reset
    site.Hooks.OnPhaseStart(s3.PhaseDiscover, func(ctx *s3.BuildContext) {
        g.tagCounts = make(map[string]int)
    })

    // Collect tags
    site.Hooks.OnResourceProcessed(func(ctx *s3.BuildContext, res *s3.Resource, targets []*s3.Resource) {
        if res.FrontMatter() == nil {
            return
        }

        tags, ok := res.FrontMatter().Data["tags"].([]any)
        if !ok {
            return
        }

        for _, tag := range tags {
            if t, ok := tag.(string); ok {
                g.tagCounts[t]++
            }
        }
    })

    // Write tag cloud data
    site.Hooks.OnPhaseEnd(s3.PhaseFinalize, func(ctx *s3.BuildContext) {
        g.writeTagCloud(ctx.Site.OutputDir)
    })
}
```

### Validation Hooks

```go
func registerValidationHooks(site *s3.Site) {
    if site.Hooks == nil {
        site.Hooks = s3.NewHookRegistry()
    }

    // Check for broken internal links
    var allURLs []string

    site.Hooks.OnResourceProcessed(func(ctx *s3.BuildContext, res *s3.Resource, targets []*s3.Resource) {
        for _, target := range targets {
            if strings.HasSuffix(target.FullPath, ".html") {
                relPath, _ := filepath.Rel(ctx.Site.OutputDir, target.FullPath)
                allURLs = append(allURLs, "/"+relPath)
            }
        }
    })

    site.Hooks.OnPhaseEnd(s3.PhaseFinalize, func(ctx *s3.BuildContext) {
        // Validate internal links
        for _, url := range allURLs {
            validateLinks(ctx.Site.OutputDir, url, allURLs)
        }
    })
}
```

## Best Practices

1. **Initialize hooks early**: Register hooks before calling `Rebuild()`

2. **Reset state at start**: Use `OnPhaseStart(PhaseDiscover, ...)` to reset collected data

3. **Write output at end**: Use `OnPhaseEnd(PhaseFinalize, ...)` to write generated files

4. **Check for nil**: Always check `res.FrontMatter()` and other nullable fields

5. **Handle errors gracefully**: Use `ctx.AddError(err)` for non-fatal errors

6. **Log progress**: Use logging to track generator activity during development

## Example: Complete Generator

```go
package main

import (
    "encoding/json"
    "os"
    "path/filepath"
    "strings"

    s3 "github.com/panyam/s3gen"
)

type ManifestGenerator struct {
    OutputPath string
    files      []ManifestEntry
}

type ManifestEntry struct {
    Path     string `json:"path"`
    Title    string `json:"title,omitempty"`
    Modified string `json:"modified"`
}

func (g *ManifestGenerator) Register(site *s3.Site) {
    if g.OutputPath == "" {
        g.OutputPath = "manifest.json"
    }

    if site.Hooks == nil {
        site.Hooks = s3.NewHookRegistry()
    }

    site.Hooks.OnPhaseStart(s3.PhaseDiscover, func(ctx *s3.BuildContext) {
        g.files = nil
    })

    site.Hooks.OnResourceProcessed(func(ctx *s3.BuildContext, res *s3.Resource, targets []*s3.Resource) {
        for _, target := range targets {
            relPath, _ := filepath.Rel(ctx.Site.OutputDir, target.FullPath)

            entry := ManifestEntry{
                Path:     "/" + relPath,
                Modified: target.UpdatedAt.Format("2006-01-02"),
            }

            if res.FrontMatter() != nil {
                if title, ok := res.FrontMatter().Data["title"].(string); ok {
                    entry.Title = title
                }
            }

            g.files = append(g.files, entry)
        }
    })

    site.Hooks.OnPhaseEnd(s3.PhaseFinalize, func(ctx *s3.BuildContext) {
        data, err := json.MarshalIndent(g.files, "", "  ")
        if err != nil {
            ctx.AddError(err)
            return
        }

        outPath := filepath.Join(ctx.Site.OutputDir, g.OutputPath)
        if err := os.WriteFile(outPath, data, 0644); err != nil {
            ctx.AddError(err)
        }
    })
}

// Usage
func main() {
    manifest := &ManifestGenerator{OutputPath: "manifest.json"}
    manifest.Register(&Site)
    Site.Rebuild(nil)
}
```
