
# Templating Guide

`s3gen` uses a powerful templating engine called `templar`, which is a superset of Go's built-in `html/template` package. This gives you the full power of Go templates, plus some extra features that make building websites easier.

## Go Template Basics

If you're new to Go templates, here are the basics:

*   **Actions**: Template actions are enclosed in `{{` and `}}`.
*   **Pipelines**: You can chain commands together using the `|` character. The output of one command becomes the input of the next.
*   **Variables**: You can store and access data using variables. The `.` character always refers to the current data object in scope.
*   **Functions**: You can call built-in and custom functions to transform your data.

For a complete guide to Go's template syntax, see the official documentation for [`html/template`](https://pkg.go.dev/html/template).

## Includes

`templar` adds an `include` directive that lets you break your templates down into smaller, reusable partials. This is great for things like headers, footers, and sidebars.

To include a template, use the `{{# include "..." #}}` syntax:

```html
{{# include "header.html" #}}

<main>
  ...
</main>

{{# include "footer.html" #}}
```

## Template Functions

`s3gen` provides a rich set of custom functions that you can use in your templates to access and manipulate your site's content. Here are some of the most common ones:

### Content Functions

*   `PagesByDate(hideDrafts bool, desc bool, offset int, count int) []*Resource`: Returns a list of pages, sorted by date.
*   `PagesByTag(tag string, hideDrafts bool, desc bool, offset int, count int) []*Resource`: Returns a list of pages that have a specific tag.
*   `LeafPages(hideDrafts bool, orderby string, offset int, count int) []*Resource`: Returns a list of "leaf" pages (i.e., pages that are not index pages).
*   `AllTags(resources []*Resource) map[string]int`: Returns a map of all tags and the number of pages that use them.
*   `json(path string, fieldpath string) (any, error)`: Reads and parses a JSON file from your `content` directory.

### Rendering Functions

*   `MDToHtml(doc *ast.Document) (template.HTML, error)`: Converts a Markdown document to HTML.
*   `RenderHtmlTemplate(templateFile, templateName string, params any) (template.HTML, error)`: Renders a Go template as HTML.
*   `RenderTextTemplate(templateFile, templateName string, params any) (string, error)`: Renders a Go template as plain text.

### Utility Functions

*   `KeysForTagMap(tagmap map[string]int, orderby string) []string`: Returns a sorted list of keys from a tag map.
*   `Slugify(text string) string`: Converts a string into a URL-friendly slug.
*   `JoinA(sep string, a []any) string`: Joins a slice of any type into a string.

## Passing Data to Templates

When a resource is rendered, `s3gen` passes a data object to the template. This object contains the following fields:

*   `.Site`: The global `Site` object, so you can access all of your site's configuration.
*   `.Res`: The current `Resource` being rendered.
*   `.FrontMatter`: The parsed front matter from the current resource.
*   `.Content`: The body content of the current resource.

You can access these fields in your templates to display dynamic content. For example, to display the title of a page, you would use `{{ .FrontMatter.title }}`.
