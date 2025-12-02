
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

## Template Functions API

`s3gen` provides a rich set of custom functions that you can use in your templates to access and manipulate your site's content.

### `AllRes`

Fetches all resources in the site, allowing you to perform site-wide operations.

*   **Signature**: `AllRes() []*Resource`
*   **Usage Example**: This is often used as a source for other functions, like `GetAllTags`.

    ```html
    {{/* Get all resources to pass to another function */}}
    {{ $allPosts := (AllRes) }}
    {{ $tagMap := (GetAllTags $allPosts) }}
    ```

### `PagesByDate`

Fetches a list of all content resources, sorted by their `date` front matter field. This is the most common function for creating a blog index or a list of recent articles.

*   **Signature**: `PagesByDate(hideDrafts bool, descending bool, offset int, count int) []*Resource`
*   **Usage Example**: Displaying the 5 most recent posts.

    ```html
    <h2>Latest Posts</h2>
    <ul>
      {{/* Get the 5 most recent posts, ignoring drafts */}}
      {{ $latestPosts := (PagesByDate true true 0 5) }}
      {{ range $latestPosts }}
        <li>
          <a href="{{ .Base.Link }}">
            {{ .Base.Title }}
          </a>
          - <span>{{ .Base.CreatedAt.Format "Jan 2, 2006" }}</span>
        </li>
      {{ else }}
        <p>No posts found!</p>
      {{ end }}
    </ul>
    ```

### `LeafPages`

Fetches a list of "leaf" pages, which are pages that are not index or list pages themselves. This is useful for getting a collection of all your individual articles or posts.

*   **Signature**: `LeafPages(hideDrafts bool, orderby string, offset int, count int) []*Resource`
*   **Usage Example**: Creating a simple list of all articles, sorted by title.
    ```html
    <h1>All Articles</h1>
    <ul>
        {{ $allArticles := (LeafPages true "title" 0 -1) }}
        {{ range $allArticles }}
            <li><a href="{{ .Base.Link }}">{{ .Base.Title }}</a></li>
        {{ end }}
    </ul>
    ```

### `GetAllTags` and `PagesByTag`

These two functions work together to create tag-based navigation. `GetAllTags` collects all unique tags from a set of resources, and `PagesByTag` fetches all pages for a single tag.

*   **`GetAllTags` Signature**: `GetAllTags(resources []*Resource) map[string]int`
*   **`PagesByTag` Signature**: `PagesByTag(tag string, hideDrafts bool, descending bool, offset int, count int) []*Resource`
*   **Usage Example**:
    1.  Create a main `tags.html` page to display a tag cloud:

        ```html
        <h1>All Tags</h1>
        {{ $allPosts := (AllRes) }}
        {{ $tagMap := (GetAllTags $allPosts) }}
        <div class="tag-cloud">
          {{ range $tag, $count := $tagMap }}
            <a href="/tags/{{ Slugify $tag }}">
              #{{ $tag }} ({{ $count }})
            </a>
          {{ end }}
        </div>
        ```

    2.  Create a parametric page at `/content/tags/[tag].html` to display posts for a specific tag:
        ```html
        {{/* This is the rendering part of the parametric page */}}
        {{ $currentTag := .Res.ParamName }}
        <h1>Posts tagged with '{{ $currentTag }}'</h1>
        <ul>
          {{ $taggedPosts := (PagesByTag $currentTag true true 0 -1) }}
          {{ range $taggedPosts }}
            <li><a href="{{ .Base.Link }}">{{ .Base.Title }}</a></li>
          {{ end }}
        </ul>
        ```

### `json`

Reads and parses a JSON file from your `content` directory. This is useful for site-wide configuration or data that you want to reuse across multiple pages.

*   **Signature**: `json(path string, fieldpath string) (any, error)`
*   **Usage Example**:
    *   **Data File**: `content/SiteMetadata.json`
        ```json
        {
          "title": "My Awesome Site",
          "author": "John Doe"
        }
        ```
    *   **Template**:
        ```html
        {{ $meta := json "SiteMetadata.json" "" }}
        <footer>
          <p>© {{ Now.Year }} {{ $meta.author }}</p>
        </footer>
        ```

### `AssetURL`

Returns the correct URL for a co-located asset file. This function handles both regular pages (where assets are co-located) and parametric pages (where assets go to a shared folder).

*   **Signature**: `AssetURL(filename string) string`
*   **Usage Example**:

    For a blog post with an image in the same directory:

    ```
    content/blog/my-post/
    ├── index.md
    └── diagram.png
    ```

    In your markdown or template:

    ```html
    <img src="{{ AssetURL "diagram.png" }}" alt="Architecture Diagram" />
    ```

    This returns:
    *   `./diagram.png` for regular pages (assets co-located with output)
    *   `/_assets/{hash}/diagram.png` for parametric pages (shared assets folder)
    *   `/static/diagram.png` as a fallback if the asset isn't found

*   **In Markdown Content**: The function is available in markdown files as well:

    ```markdown
    Check out this diagram:

    <img src="{{ AssetURL "chart.svg" }}" alt="Chart" />
    ```

### `StageSet` and `StageGet`

These functions allow you to pass data between templates within a single render. Useful for complex template hierarchies.

*   **`StageSet`**: Stores a value that can be retrieved later.
*   **`StageGet`**: Retrieves a previously stored value.

*   **Usage Example**:

    ```html
    {{/* In PostSimple.html - parse markdown and store the document */}}
    {{ $parsed := ParseMD .Content }}
    {{ StageSet "Document" $parsed.Doc "TOC" $parsed.TOC }}

    {{/* Later in Article.html - retrieve and render */}}
    {{ $doc := StageGet "Document" }}
    {{ MDToHtml $doc }}
    ```

## Passing Data to Templates

When a resource is rendered, `s3gen` passes a data object to the template. This object contains the following fields:

*   `.Site`: The global `Site` object, so you can access all of your site's configuration.
*   `.Res`: The current `Resource` being rendered.
*   `.FrontMatter`: The parsed front matter from the current resource.
*   `.Content`: The body content of the current resource.

You can access these fields in your templates to display dynamic content. For example, to display the title of a page, you would use `{{ .FrontMatter.title }}`.
