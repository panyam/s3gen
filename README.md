
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
*   **Powerful Go-based templating**: with includes, custom functions, and more.
*   **Parametric Routes**: Generate multiple pages from a single template (e.g., for tags or categories).
*   **Live Reloading** for rapid development.
*   **Extensible**: Add your own content types and build rules.
*   **Library-first**: Use it as a standalone CLI or embed it in your own Go application.

## Examples

[My Personal Blog](httpss://buildmage.com)

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
}

func main() {
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
    {{ .Content | HTML }}
</body>
</html>
```

To run the build:

```bash
go run main.go
```

## Where to Go Next

For more detailed documentation, please see the `/docs` directory.

## Contributing & License

Contributions are welcome! Please open an issue or submit a pull request.

This project is licensed under the [MIT License](LICENSE).
