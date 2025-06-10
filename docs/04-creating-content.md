
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

A parametric page is a template that can generate multiple pages from a single file. This is useful for things like tag and category pages, where the layout is the same but the content is different for each term.

To create a parametric page, you create a file with a name like `[param].html`. For example, a tag page might be at `content/tags/[tag].html`.

Inside this template, you'll have two main sections:

1.  **Parameter Discovery**: The first part of the template is responsible for discovering all the possible values for the parameter. You'll typically use `s3gen`'s template functions to get a list of all your content and then extract the unique terms (e.g., all the unique tags). For each term, you'll call the `AddParam` function to tell `s3gen` to generate a page for that term.
2.  **Page Rendering**: The second part of the template is responsible for rendering the page for a *specific* parameter value. `s3gen` will re-run the template for each parameter value it discovered, and in this second pass, you can access the current parameter value using `.Res.ParamName`.

Here's a simplified example of a tag page at `content/tags/[tag].html`:

```html
---
title: "Tags"
---

{{ if eq .Res.ParamName "" }}
  {{/* Parameter Discovery Phase */}}
  {{ $posts := PagesByDate false true 0 -1 }}
  {{ $tagmap := AllTags $posts }}
  {{ $sortedTags := KeysForTagMap $tagmap "-count" }}
  {{ range $sortedTags }}
    {{ .Res.AddParam (Slugify .) }}
  {{ end }}
{{ else }}
  {{/* Page Rendering Phase */}}
  <h1>Posts tagged with "{{ .Res.ParamName }}"</h1>
  {{ $posts := PagesByTag .Res.ParamName false true 0 -1 }}
  <ul>
    {{ range $posts }}
      <li><a href="{{ .Base.Link }}">{{ .Base.Title }}</a></li>
    {{ end }}
  </ul>
{{ end }}
```

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
