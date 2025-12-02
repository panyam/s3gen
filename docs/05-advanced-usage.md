
# Advanced Usage

Once you've mastered the basics of `s3gen`, you can start to explore some of its more advanced features.

## Live Reloading

`s3gen` comes with a built-in live-reloading server that you can use for development. To enable it, you just need to call the `Watch()` method on your `Site` object.

Here's a common pattern for a `main.go` file that uses live reloading in development and runs a single build in production:

```go
package main

import (
	"log"
	"os"

	s3 "github.com/panyam/s3gen"
)

var Site = s3.Site{
	// ... your site configuration
}

func main() {
	if os.Getenv("APP_ENV") != "production" {
		log.Println("Starting watcher...")
		Site.Watch()
		// Keep the program running
		select {}
	} else {
		Site.Rebuild(nil)
	}
}
```

Now, when you run `go run main.go`, `s3gen` will start a server and watch your `content` and `templates` directories for changes. When you save a file, it will automatically rebuild your site.

## Configuring Build Rules

The `Site.BuildRules` slice defines the pipeline for processing your content. The order of rules in this slice is critical, as `s3gen` will use the **first rule** that successfully matches a resource.

This allows you to prioritize specialized rules over more general ones. A key example of this is the `ParametricPages` rule, which must be configured to run *before* the standard `MDToHtml` and `HTMLToHtml` rules.

Here is an example of a well-structured `BuildRules` configuration in a `site.go` file:

```go
// In your site's main.go or site.go
var Site = s3.Site{
    // ... other config
    BuildRules: []s3gen.Rule{
        // 1. The ParametricPages rule comes first. It is configured with a
        //    map of renderers, telling it which specialized rule to use for
        //    the final rendering step based on the file extension.
        &s3gen.ParametricPages{
            Renderers: map[string]s3gen.Rule{
                ".md":  &s3gen.MDToHtml{BaseToHtmlRule: s3gen.BaseToHtmlRule{Extensions: []string{".md"}}},
                ".mdx": &s3gen.MDToHtml{BaseToHtmlRule: s3gen.BaseToHtmlRule{Extensions: []string{".mdx"}}},
                ".html": &s3gen.HTMLToHtml{BaseToHtmlRule: s3gen.BaseToHtmlRule{Extensions: []string{".html"}}},
            },
        },

        // 2. The standard rules come next. These will only be used for
        //    resources that were NOT matched by the ParametricPages rule.
        &s3gen.MDToHtml{BaseToHtmlRule: s3gen.BaseToHtmlRule{Extensions: []string{".md", ".mdx"}}},
        &s3gen.HTMLToHtml{BaseToHtmlRule: s3gen.BaseToHtmlRule{Extensions: []string{".html", ".htm"}}},
    },
    // ... other config
}
```

By placing `ParametricPages` first, you ensure that files like `[tag].md` or `[category].html` are correctly identified and handled by the parametric engine. Any regular, non-parametric `.md` or `.html` files will not be matched by the `ParametricPages` rule and will fall through to be processed by the standard rules.

## Programmatic Use

Because `s3gen` is a library first, you can easily embed it into a larger Go application. This is useful if you want to serve your static site from the same binary as your API or other web services.

The `Site` object has a `Handler()` method that returns an `http.Handler`. You can use this to serve your generated site with Go's standard `http` server.

```go
package main

import (
	"log"
	"net/http"

	s3 "github.com/panyam/s3gen"
)

var Site = s3.Site{
	// ... your site configuration
}

func main() {
	// Rebuild the site once on startup
	Site.Rebuild(nil)

	// Create a new ServeMux
	mux := http.NewServeMux()

	// Mount the s3gen handler at the root
	mux.Handle("/", Site.Handler())

	// You can add other handlers to the mux
	mux.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	log.Println("Starting server on :8080")
	http.ListenAndServe(":8080", mux)
}
```

## Custom Rules

The real power of `s3gen` comes from its extensible, rule-based build system. You can create your own rules to process any file format you need.

### The Basic Rule Interface

To create a custom rule, implement the `s3gen.Rule` interface:

```go
type Rule interface {
	// Determines if the rule can handle a resource and what the output target should be.
	TargetsFor(site *Site, res *Resource) (siblings []*Resource, targets []*Resource)

	// Processes the resource and writes the output.
	Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error
}
```

### The PhaseRule Interface

For more control over when your rule runs, implement `PhaseRule`:

```go
type PhaseRule interface {
	Rule

	// Phase returns which build phase this rule runs in.
	// Options: PhaseTransform, PhaseGenerate, PhaseFinalize
	Phase() BuildPhase

	// DependsOn returns glob patterns of files this rule needs as input.
	// Used to order rules within a phase.
	DependsOn() []string

	// Produces returns glob patterns of files this rule creates.
	// Used to order rules within a phase.
	Produces() []string
}
```

### The AssetAwareRule Interface

If your rule handles content files with co-located assets, implement `AssetAwareRule`:

```go
type AssetAwareRule interface {
	Rule

	// HandleAssets is called with co-located assets for a resource.
	// Returns mappings describing how each asset should be processed.
	HandleAssets(site *Site, res *Resource, assets []*Resource) ([]AssetMapping, error)
}

type AssetMapping struct {
	Source *Resource
	Dest   string      // Path relative to output directory
	Action AssetAction // AssetCopy, AssetProcess, or AssetSkip
}
```

### Example: A Transform Phase Rule

Here's an example of a rule that runs in the Transform phase to process SCSS files:

```go
package main

import (
	"os/exec"
	"path/filepath"
	"strings"

	s3 "github.com/panyam/s3gen"
)

type SCSSRule struct{}

// Phase declares this runs in the Transform phase
func (s *SCSSRule) Phase() s3.BuildPhase {
	return s3.PhaseTransform
}

// DependsOn declares what files this rule needs
func (s *SCSSRule) DependsOn() []string {
	return []string{"**/*.scss"}
}

// Produces declares what files this rule creates
func (s *SCSSRule) Produces() []string {
	return []string{"**/*.css"}
}

func (s *SCSSRule) TargetsFor(site *s3.Site, res *s3.Resource) ([]*s3.Resource, []*s3.Resource) {
	if !strings.HasSuffix(res.FullPath, ".scss") {
		return nil, nil
	}

	// The output path will be the same as the input path, but with a .css extension
	outPath := strings.TrimSuffix(res.FullPath, ".scss") + ".css"
	relPath, _ := filepath.Rel(site.ContentRoot, outPath)
	destPath := filepath.Join(site.OutputDir, relPath)

	target := site.GetResource(destPath)
	target.Source = res

	return []*s3.Resource{res}, []*s3.Resource{target}
}

func (s *SCSSRule) Run(site *s3.Site, inputs []*s3.Resource, targets []*s3.Resource, funcs map[string]any) error {
	input := inputs[0]
	output := targets[0]
	output.EnsureDir()

	cmd := exec.Command("sass", "--no-source-map", input.FullPath, output.FullPath)
	return cmd.Run()
}
```

### Example: Using Built-in Transform Rules

`s3gen` provides several built-in transform rules:

```go
import s3 "github.com/panyam/s3gen"

var Site = s3.Site{
	BuildRules: []s3.Rule{
		// Minify CSS files (creates .min.css versions)
		&s3.CSSMinifier{
			OutputSuffix:    ".min",
			ExcludePatterns: []string{"*.min.css"},
		},

		// Compile SCSS to CSS using external sass command
		s3.NewSCSSTransform(),

		// Compile TypeScript using esbuild
		s3.NewTypeScriptTransform(),

		// Copy static files
		&s3.CopyRule{
			Patterns: []string{"*.woff", "*.woff2", "*.ttf"},
		},

		// ... other rules
	},
}
```

### Legacy Rule Compatibility

If you have existing rules that don't implement `PhaseRule`, they'll still work. `s3gen` automatically wraps them with `LegacyRuleAdapter`, which runs them in the Generate phase.

## Using Generators

Generators produce site-wide artifacts like sitemaps and RSS feeds. They use hooks to collect data during the build and write output in the Finalize phase.

```go
import s3 "github.com/panyam/s3gen"

// Create generators
var sitemapGen = &s3.SitemapGenerator{
	BaseURL:    "https://example.com",
	OutputPath: "sitemap.xml",
	ChangeFreq: "weekly",
	Priority:   0.5,
}

var rssGen = &s3.RSSGenerator{
	Title:       "My Blog",
	Description: "Latest posts from my blog",
	BaseURL:     "https://example.com",
	OutputPath:  "feed.xml",
	MaxItems:    20,
}

func main() {
	// Register generators with the site
	sitemapGen.Register(&Site)
	rssGen.Register(&Site)

	// Now run the build
	Site.Rebuild(nil)
}
```

See [Generators and Hooks](09-generators-and-hooks.md) for more details on creating custom generators.

## Co-Located Assets

`s3gen` supports placing assets (images, data files) next to your content files. Configure which files are treated as assets:

```go
var Site = s3.Site{
	AssetPatterns: []string{
		"*.png", "*.jpg", "*.jpeg", "*.gif", "*.svg", "*.webp",
		"*.mp4", "*.webm", "*.pdf",
	},
}
```

Assets are automatically copied to the output directory alongside the generated HTML. For parametric pages, assets are deduplicated using content hashes.

See [Co-Located Assets](07-co-located-assets.md) for full documentation.
