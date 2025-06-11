
# Creating Content

In `s3gen`, your website's content lives in the `content` directory. You can create content in a variety of formats, but the most common are Markdown and HTML.

## Front Matter

`s3gen` uses front matter to add metadata to your content. Front matter is a block of YAML or TOML at the top of your file, enclosed in `---` (for YAML) or `+++` (for TOML).

Here's an example of some YAML front matter in a Markdown file:

```markdown
---
title: "My First Blog Post"
date: "2023-10-27"
tags: ["go", "s3gen"]
template: "post.html"
---

This is the content of my blog post.
```

You can access this metadata in your templates using the `.FrontMatter` object. For example, to display the title of the page, you would use `{{ .FrontMatter.title }}`.

## Specifying a Page Template

One of the most powerful features of the front matter is the `template` key. This allows you to specify which top-level template file from your `templates` directory should be used to render the content.

This gives you the flexibility to create different layouts for different types of content. For example, a blog post might use a different base template than a project page or the homepage.

**Example:**

Imagine you have two main layouts in your `templates` directory: `BlogPage.html` and `ProjectPage.html`.

For a blog post, your front matter would look like this:

```markdown
---
title: "My Blog Post"
date: "2023-10-27"
template: "BlogPage.html"
---

Blog post content here...
```

And for a project page, it would look like this:

```markdown
---
title: "My Awesome Project"
date: "2023-10-28"
template: "ProjectPage.html"
---

Project page content here...
```

When `s3gen` builds the site, it will use the specified template as the entry point for rendering, allowing you to have completely different HTML structures, CSS, and JavaScript for each type of page. If no `template` is specified, `s3gen` will use the `DefaultBaseTemplate` defined in your `Site` configuration.

## Basic Pages

A basic page is any content file that is not an index page or a parametric page. These are typically your blog posts, "about" pages, and other individual pieces of content.

To create a basic page, just create a new `.html` or `.md` file in your `content` directory. `s3gen` will process it using the appropriate rule and render it to a corresponding file in your `OutputDir`.

By default, a file at `content/a/b/c.md` will be rendered to `public/a/b/c/index.html`. This can be configured by creating custom rules.

## List Pages

A list page is a page that displays a list of other pages. A common example is a blog index page that lists all of your blog posts.

To create a list page, you'll use `s3gen`'s template functions to fetch a list of resources and then loop through them in your template.

Here's an example of how you could create a blog index page:

```html
---
title: "My Blog"
---

<h1>My Blog</h1>

<ul>
  {{ $posts := PagesByDate false true 0 -1 }}
  {{ range $posts }}
    <li>
      <a href="{{ .Base.Link }}">{{ .Base.Title }}</a>
    </li>
  {{ end }}
</ul>
```

## Parametric Pages

A parametric page is a template that can generate multiple pages from a single file. This is useful for things like tag and category pages, where the layout is the same but the content is different for each term. Parametric pages are handled by the built-in `ParametricPages` rule.

To create a parametric page, you create a file with a name like `[param].html` or `[param].md`. For example, a tag page might be at `content/tags/[tag].html`.

The `ParametricPages` rule works in two phases, and your template must support both:

1.  **Discovery Phase**: `s3gen` first "pre-renders" the template to discover all possible values for the parameter. In this phase, the `.Res.ParamName` variable is an empty string. Your template logic should call the `AddParam` function for each value it finds (e.g., for each unique tag).
2.  **Generation Phase**: After discovering the values, `s3gen` renders the template once for each value. In this phase, `.Res.ParamName` is set to the specific value for that page (e.g., "go", "react", etc.). Your template logic should use this value to filter and display the correct content.

Here's a simplified example of a tag page at `content/tags/[tag].html`:

```html
---
title: "Tags"
---

{{/* Check if we are in the discovery or generation phase */}}
{{ if eq .Res.ParamName "" }}

  {{/* --- Discovery Phase --- */}}
  {{/* Find all unique tags and add them as parameters */}}
  {{ $posts := (AllRes) }}
  {{ $tagmap := (GetAllTags $posts) }}
  {{ $sortedTags := (KeysForTagMap $tagmap "-count") }}
  {{ range $sortedTags }}
    {{ $.Res.AddParam . }}
  {{ end }}

{{ else }}

  {{/* --- Generation Phase --- */}}
  {{ $currentTag := .Res.ParamName }}
  <h1>Posts tagged with "{{ $currentTag }}"</h1>
  
  {{ $posts := (PagesByTag $currentTag false true 0 -1) }}
  <ul>
    {{ range $posts }}
      <li><a href="{{ .Base.Link }}">{{ .Base.Title }}</a></li>
    {{ end }}
  </ul>

{{ end }}
```

This two-phase model allows you to dynamically generate pages based on your content without any manual configuration.

## Using JSON Data

You can store data in `.json` files in your `content` directory and access it from any template using the `json` function. This is useful for site-wide configuration or data that you want to reuse across multiple pages.

For example, you could have a `content/SiteMetadata.json` file:

```json
{
  "title": "My Awesome Site",
  "author": "John Doe"
}
```

And then access it in a template like this:

```html
{{ $meta := json "SiteMetadata.json" "" }}
<h1>{{ $meta.title }}</h1>
<p>By {{ $meta.author }}</p>
```
