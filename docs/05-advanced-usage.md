
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

To create a custom rule, you need to implement the `s3gen.Rule` interface:

```go
type Rule interface {
	// Determines if the rule can handle a resource and what the output target should be.
	TargetsFor(site *Site, res *Resource) (siblings []*Resource, targets []*Resource)

	// Processes the resource and writes the output.
	Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error
}
```

Here's a simplified example of a rule that could process SASS files:

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	s3 "github.com/panyam/s3gen"
)

type SassRule struct{}

func (s *SassRule) TargetsFor(site *s3.Site, res *s3.Resource) ([]*s3.Resource, []*s3.Resource) {
	if res.Ext() != ".scss" {
		return nil, nil
	}

	// The output path will be the same as the input path, but with a .css extension
	destpath := strings.TrimSuffix(res.FullPath, ".scss") + ".css"
	target := site.GetResource(filepath.Join(site.OutputDir, destpath))
	target.Source = res

	return []*s3.Resource{res}, []*s3.Resource{target}
}

func (s *SassRule) Run(site *s3.Site, inputs []*s3.Resource, targets []*s3.Resource, funcs map[string]any) error {
	inPath := inputs[0].FullPath
	outPath := targets[0].FullPath

	cmd := exec.Command("sass", inPath, outPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func main() {
	var Site = s3.Site{
		BuildRules: []s3.Rule{
			&SassRule{},
			// ... other rules
		},
		// ... other site configuration
	}

	Site.Rebuild(nil)
}
```

This is a simple example, but it demonstrates the power and flexibility of the rule-based system. You can use this pattern to integrate any build tool or process into your `s3gen` site.
