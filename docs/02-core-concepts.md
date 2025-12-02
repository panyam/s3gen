
# Core Concepts

`s3gen`'s architecture is built around a few core concepts. Understanding these will help you to get the most out of the tool and build your site effectively.

## The `Site` Object

The `s3gen.Site` object is the central nervous system of your project. It's a Go struct that holds all the configuration for your website. You define your site's structure, build rules, and templating options here.

Here's a breakdown of some of the most important fields:

*   `ContentRoot`: The directory where all your content files are stored.
*   `OutputDir`: The directory where `s3gen` will write the generated static files.
*   `TemplateFolders`: A list of directories where your Go templates are located.
*   `BuildRules`: A slice of `Rule` objects that define how to process different types of files.
*   `PhaseRules`: Rules organized by build phase (Transform, Generate, Finalize).
*   `Hooks`: A registry of callbacks for observing and reacting to build events.
*   `AssetPatterns`: Glob patterns defining co-located assets (e.g., `["*.png", "*.jpg"]`).
*   `PriorityFunc`: A function that determines the order in which resources are processed within a phase.
*   `LiveReload`: A boolean that enables or disables live reloading during development.

## Resources

In `s3gen`, every file in your `ContentRoot` is treated as a `Resource`. A `Resource` is a Go struct that contains information about the file, including its path, content, front matter, and current state in the build process (e.g., `Pending`, `Loaded`, `Failed`).

Resources can also track relationships:

*   `Assets`: Co-located files (images, data files) associated with this resource.
*   `AssetOf`: If this resource is an asset, points to the parent content resource.
*   `ProducedBy`: Which rule created this resource (for generated targets).
*   `ProducedAt`: Which build phase this resource was created in.

This abstraction allows `s3gen` to work with different file types in a consistent way and provides a structured way to manage your content throughout the build process.

## Rules

A `Rule` is a Go interface that defines a set of actions to be performed on a `Resource`. It's the heart of `s3gen`'s extensibility. Each rule has two main methods:

*   `TargetsFor(site *Site, res *Resource)`: This method determines if the rule can be applied to a given resource and, if so, what the output file (or "target") should be.
*   `Run(site *Site, inputs []*Resource, targets []*Resource, ...)`: This method contains the logic for processing the resource. For example, the `MDToHtml` rule's `Run` method converts Markdown to HTML.

`s3gen` comes with a set of built-in rules for common tasks like processing Markdown and HTML files, but you can easily create your own to handle any file type you need.

### Rule Interfaces

Rules can implement additional interfaces for more control:

*   **`PhaseRule`**: Declares which build phase the rule runs in, and what files it depends on and produces.
*   **`AssetAwareRule`**: Handles co-located assets (images, data files) associated with content.

See [Advanced Usage](05-advanced-usage.md) and [Phase-Based Architecture](06-phase-based-architecture.md) for details.

## The Build Process

When you run a build, `s3gen` executes a **four-phase pipeline**:

```
Discover → Transform → Generate → Finalize
```

### Phase 1: Discover

*   Walks your `ContentRoot` directory to find all files
*   Creates a `Resource` object for each file
*   Loads front matter and metadata
*   Identifies co-located assets based on `AssetPatterns`
*   Sorts resources by `PriorityFunc` for processing order

### Phase 2: Transform

*   Runs transformation rules on assets and source files
*   Examples: SCSS → CSS compilation, CSS minification, image optimization
*   Transform rules implement `PhaseRule` with `Phase() = PhaseTransform`

### Phase 3: Generate

*   Produces final output files from content
*   Markdown → HTML conversion
*   HTML template processing
*   Parametric page expansion (e.g., `[tag].html` → `go/index.html`, `rust/index.html`)
*   Most content rules run in this phase

### Phase 4: Finalize

*   Runs after all content is generated
*   Generates site-wide artifacts: sitemap.xml, RSS feeds, search indexes
*   Finalize rules have access to all generated targets via `BuildContext`

### Build Context

During a build, a `BuildContext` struct is passed through all phases:

```go
type BuildContext struct {
    Site             *Site
    CurrentPhase     BuildPhase
    Resources        []*Resource      // All discovered resources
    GeneratedTargets []*Resource      // All targets created so far
    Errors           []error          // Non-fatal errors
}
```

This allows rules and hooks to access global build state.

## Hooks

Hooks provide a way to observe and react to build events without implementing a full rule. They're perfect for generators and side-effects:

```go
site.Hooks.OnPhaseStart(PhaseDiscover, func(ctx *BuildContext) {
    // Called when Discover phase starts
})

site.Hooks.OnPhaseEnd(PhaseFinalize, func(ctx *BuildContext) {
    // Called when Finalize phase ends - write sitemap, etc.
})

site.Hooks.OnResourceProcessed(func(ctx *BuildContext, res *Resource, targets []*Resource) {
    // Called after each resource is processed
})
```

See [Generators and Hooks](09-generators-and-hooks.md) for more details.

## Co-Located Assets

Assets like images and data files can be placed next to your content files. `s3gen` will automatically discover them and copy them to the output directory alongside the generated HTML.

```
content/
├── blog/
│   └── my-post/
│       ├── index.md        → output/blog/my-post/index.html
│       ├── diagram.png     → output/blog/my-post/diagram.png
│       └── chart.svg       → output/blog/my-post/chart.svg
```

Configure asset patterns in your site:

```go
site := s3.Site{
    AssetPatterns: []string{"*.png", "*.jpg", "*.svg", "*.gif"},
}
```

Reference assets in your content with relative paths:

```markdown
![Diagram](./diagram.png)
```

Or use the `AssetURL` template function for more control:

```html
<img src="{{ AssetURL "diagram.png" }}" alt="Diagram" />
```

See [Co-Located Assets](07-co-located-assets.md) for full documentation.
