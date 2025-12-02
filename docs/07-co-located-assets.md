
# Co-Located Assets

Co-located assets allow you to place images, data files, and other resources next to your content files. `s3gen` will automatically discover these assets and copy them to the output directory alongside the generated HTML.

## Why Co-Located Assets?

Traditional static site generators often require you to put all images in a central `/static/images/` folder and reference them with absolute paths. This has drawbacks:

- Images are disconnected from the content that uses them
- Harder to organize related files together
- More difficult to move or rename content

With co-located assets:

```
content/
├── blog/
│   └── my-post/
│       ├── index.md        # Your content
│       ├── diagram.png     # Referenced as ./diagram.png
│       ├── chart.svg       # Referenced as ./chart.svg
│       └── data.json       # Available to templates
```

## Configuration

Enable co-located assets by specifying which file patterns should be treated as assets:

```go
var Site = s3.Site{
    ContentRoot: "./content",
    OutputDir:   "./output",

    // Define which files are assets
    AssetPatterns: []string{
        "*.png", "*.jpg", "*.jpeg", "*.gif", "*.svg", "*.webp",
        "*.mp4", "*.webm",
        "*.pdf",
    },

    // ... other config
}
```

## How It Works

### Discovery

During the Discover phase, `s3gen`:

1. Identifies content files (`.md`, `.html`) in your content directory
2. Looks for files matching `AssetPatterns` in the same directory
3. Associates those files as assets of the content resource

### Copying

During the Generate phase, when a content file is processed:

1. The content is converted to HTML and written to the output directory
2. Associated assets are copied to the same output directory
3. Relative paths in the content (like `./diagram.png`) just work

### Output Structure

```
content/                          output/
├── blog/                         ├── blog/
│   └── my-post/                  │   └── my-post/
│       ├── index.md         →    │       ├── index.html
│       ├── diagram.png      →    │       ├── diagram.png
│       └── chart.svg        →    │       └── chart.svg
```

## Using Assets in Content

### In Markdown

Standard markdown image syntax with relative paths:

```markdown
# My Post

Here's a diagram:

![Architecture Diagram](./diagram.png)

And a chart:

![Sales Chart](./chart.svg)
```

### Using AssetURL Function

For more control, use the `AssetURL` template function:

```markdown
# My Post

<img src="{{ AssetURL "diagram.png" }}" alt="Diagram" class="full-width" />
```

The `AssetURL` function:
- Returns `./diagram.png` for regular pages
- Returns `/_assets/{hash}/diagram.png` for parametric pages
- Falls back to `/static/diagram.png` if the asset isn't found

### In HTML Templates

```html
{{/* Access assets via the template function */}}
<figure>
    <img src="{{ AssetURL "screenshot.png" }}" alt="Screenshot" />
    <figcaption>Application screenshot</figcaption>
</figure>
```

## Parametric Pages and Shared Assets

Parametric pages (like `[tag].html` that generates `/tags/go/`, `/tags/rust/`, etc.) present a challenge: the same assets would be duplicated for each generated page.

`s3gen` solves this with **shared assets**:

1. Assets for parametric pages go to a `_assets/` directory
2. Each asset is stored under a content hash to enable deduplication
3. The `AssetURL` function returns the correct shared path

### Example

For a parametric tag page with assets:

```
content/
├── tags/
│   └── [tag].html          # Parametric template
│   └── badge.png           # Shared asset
```

Output:

```
output/
├── _assets/
│   └── a1b2c3d4/           # Content hash prefix
│       └── badge.png       # Single copy
├── tags/
│   ├── go/
│   │   └── index.html      # References /_assets/a1b2c3d4/badge.png
│   ├── rust/
│   │   └── index.html      # References same asset
│   └── python/
│       └── index.html      # References same asset
```

## Per-Resource Asset Patterns

You can override site-level patterns for specific resources using front matter:

```yaml
---
title: My Post
assets:
  - "*.png"
  - "*.json"
  - "custom-data/*.csv"
---
```

This resource will only pick up files matching these patterns as assets.

## The AssetAwareRule Interface

Rules that handle content files can implement `AssetAwareRule` to customize asset handling:

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
    Action AssetAction
}

type AssetAction int

const (
    AssetCopy    AssetAction = iota  // Copy as-is
    AssetProcess                      // Run through transform rules
    AssetSkip                         // Don't copy
)
```

### Example Custom Handler

```go
func (m *MyRule) HandleAssets(site *Site, res *Resource, assets []*Resource) ([]AssetMapping, error) {
    var mappings []AssetMapping

    for _, asset := range assets {
        action := AssetCopy

        // Skip large videos
        if strings.HasSuffix(asset.FullPath, ".mp4") {
            info, _ := os.Stat(asset.FullPath)
            if info.Size() > 100*1024*1024 { // > 100MB
                action = AssetSkip
            }
        }

        mappings = append(mappings, AssetMapping{
            Source: asset,
            Dest:   filepath.Join(getOutputDir(res), filepath.Base(asset.FullPath)),
            Action: action,
        })
    }

    return mappings, nil
}
```

## API Reference

### Site Configuration

```go
type Site struct {
    // AssetPatterns are glob patterns for files treated as assets.
    // Examples: "*.png", "*.jpg", "images/*"
    AssetPatterns []string

    // SharedAssetsDir is the directory for parametric page assets.
    // Default: "_assets"
    SharedAssetsDir string
}
```

### Resource Fields

```go
type Resource struct {
    // Assets are co-located files associated with this resource.
    Assets []*Resource

    // AssetOf points to the parent content resource if this is an asset.
    AssetOf *Resource
}
```

### Template Functions

```go
// AssetURL returns the URL for a co-located asset.
// Available in both templates and markdown content.
AssetURL(filename string) string
```

### Helper Functions

```go
// GetAssetURL returns the URL for an asset relative to a resource.
func GetAssetURL(site *Site, res *Resource, filename string) string

// ContentHashShort returns the first 8 characters of a file's SHA256 hash.
// Used for asset deduplication.
func ContentHashShort(res *Resource) string
```

## Best Practices

1. **Use descriptive filenames**: `architecture-diagram.png` is better than `img1.png`

2. **Keep assets small**: Optimize images before adding them to your content

3. **Use relative paths**: Always use `./filename` in your content for portability

4. **Use AssetURL for conditional logic**: When you need different behavior for parametric pages

5. **Configure patterns precisely**: Only include file types you actually use as assets
