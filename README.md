
# s3gen

A simple, flexible, rule-based static site generator for Go developers.

S3Gen is a simple static site generator written in Go. It lets you do some frequent things very easily:

1.  Compile your posts and content written in markdown (.md) into html files to be served statically.
2.  Use Go templating and a rich set of "builtin" functions for you to use.
3.  Use custom templates for different pages in your website/blog.
4.  Generates an RSS feed so your site can be consumed via a feed reader
5.  Generates a sitemap useable by search engines
6.  Library mode to programatically serve static sites inside another binary

## Key Features

*   **Rule-based build system**: Process files exactly how you want.
*   **Four-phase build pipeline**: Discover → Transform → Generate → Finalize
*   **Powerful Go-based templating**: with includes, custom functions, and more.
*   **Parametric Routes**: Generate multiple pages from a single template (e.g., for tags or categories).
*   **Co-located Assets**: Place images next to your markdown files.
*   **Transform Rules**: SCSS, TypeScript, CSS minification, and more.
*   **Generators**: Sitemap, RSS feed, and custom site-wide artifacts.
*   **Live Reloading** for rapid development.
*   **Extensible**: Add your own content types and build rules.
*   **Library-first**: Use it as a standalone CLI or embed it in your own Go application.

## Examples

[My Personal Blog](https://buildmage.com) - [Source](https://github.com/panyam/blog)

## Getting Started

### With a Go compiler

```
go install github.com/panyam/s3gen@latest
```

### Precompiled Binary

TBD

## Setup and Usage

This library is mainly for developers who are interested in hosting a blog but want control and customization easily. There are very few conventions needed for this.

A typical `s3gen` project has a `content` directory for the site's content, a `templates` directory for the Go templates, and a `static` directory for assets like CSS, JavaScript, and images. A `main.go` file configures the `s3gen.Site` object.

A minimal `main.go` showing how to configure and run a `s3gen.Site`:

```go
package main

import (
	"log"
	"os"

	s3 "github.com/panyam/s3gen"
)

var Site = s3.Site{
	OutputDir:   "./public",
	ContentRoot: "./content",
	PathPrefix:  "/",
	TemplateFolders: []string{
		"./templates",
	},
	StaticFolders: []string{
		"/static/", "static",
	},
	DefaultBaseTemplate: s3.BaseTemplate{
		Name: "base.html",
	},
	// Co-located asset patterns
	AssetPatterns: []string{
		"*.png", "*.jpg", "*.jpeg", "*.gif", "*.svg",
	},
}

// Generators for site-wide artifacts
var sitemapGen = &s3.SitemapGenerator{
	BaseURL:    "https://example.com",
	OutputPath: "sitemap.xml",
}

var rssGen = &s3.RSSGenerator{
	Title:       "My Blog",
	Description: "Latest posts",
	BaseURL:     "https://example.com",
	OutputPath:  "feed.xml",
}

func main() {
	// Register generators
	sitemapGen.Register(&Site)
	rssGen.Register(&Site)

	if os.Getenv("APP_ENV") != "production" {
		log.Println("Starting watcher...")
		Site.Watch()
		select {}
	} else {
		Site.Rebuild(nil)
	}
}
```

A minimal `content/index.html` with some front matter:

```html
---
title: "Home"
---

<h1>Welcome to my website!</h1>
```

A minimal `templates/base.html` to render the content:

```html
<!DOCTYPE html>
<html>
<head>
    <title>{{.FrontMatter.title}}</title>
</head>
<body>
     {{ BytesToString .Content | HTML }}
</body>
</html>
```

To run the build:

```bash
go run main.go
```

## Documentation

For detailed documentation, see the `/docs` directory:

| Document | Description |
|----------|-------------|
| [01-introduction.md](docs/01-introduction.md) | Philosophy and use cases |
| [02-core-concepts.md](docs/02-core-concepts.md) | Site, Resources, Rules, Build Process |
| [03-templating-guide.md](docs/03-templating-guide.md) | Template functions and syntax |
| [04-creating-content.md](docs/04-creating-content.md) | Front matter, parametric pages, JSON data |
| [05-advanced-usage.md](docs/05-advanced-usage.md) | Custom rules, programmatic use |
| [06-phase-based-architecture.md](docs/06-phase-based-architecture.md) | The four-phase build pipeline |
| [07-co-located-assets.md](docs/07-co-located-assets.md) | Images and files next to content |
| [08-transform-rules.md](docs/08-transform-rules.md) | CSS minification, SCSS, TypeScript |
| [09-generators-and-hooks.md](docs/09-generators-and-hooks.md) | Sitemap, RSS, custom generators |

## Architecture Overview

s3gen uses a four-phase build pipeline:

```
Discover → Transform → Generate → Finalize
    │          │           │          │
    │          │           │          └─ Sitemap, RSS, search index
    │          │           └─ MD→HTML, HTML→HTML, parametric expansion
    │          └─ SCSS→CSS, image optimization, bundling
    └─ Find all resources, identify types, load metadata
```

### Built-in Rules

**Transform Phase:**
- `CSSMinifier` - Minify CSS files
- `ExternalTransform` - Run any command-line tool
- `CopyRule` - Copy static files
- `NewSCSSTransform()` - SCSS compilation
- `NewTypeScriptTransform()` - TypeScript compilation

**Generate Phase:**
- `MDToHtml` - Markdown to HTML
- `HTMLToHtml` - HTML template processing
- `ParametricPages` - Generate multiple pages from one template

**Finalize Phase:**
- `SitemapGenerator` - Generate sitemap.xml
- `RSSGenerator` - Generate RSS feed

## Contributing & License

Contributions are welcome! Please open an issue or submit a pull request.

This project is licensed under the [MIT License](LICENSE).
