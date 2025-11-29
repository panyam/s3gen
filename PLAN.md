# S3Gen Multi-Rule Architecture Plan

## Status: Phase 1 Complete ✓

**Last Updated:** 2025-11-29

## Overview

This plan extends s3gen with a phase-based architecture that enables:
1. Transformation pipelines (SCSS→CSS→minify)
2. Side-effect generators (sitemap, RSS, search index)
3. Multi-output rules (same resource → multiple outputs)
4. Co-located assets (images next to markdown files)

## Implementation Progress

### ✅ Completed (Phase 1 - Core Architecture)

| Step | Description | Status |
|------|-------------|--------|
| 1 | Core Types in `phase.go` | ✅ Done |
| 2 | Resource Enhancement in `resource.go` | ✅ Done |
| 3 | Site Changes in `site.go` | ✅ Done |
| 4 | Asset Utilities in `assets.go` | ✅ Done |
| 5 | Update Existing Rules | ✅ Done |

### ✅ Completed (Phase 2 - Generators)

| Step | Description | Status |
|------|-------------|--------|
| 6 | Example Generators (sitemap, RSS) | ✅ Done |

### ✅ Completed (Phase 3 - Transform Rules)

| Step | Description | Status |
|------|-------------|--------|
| 7 | Transform Rules (CSSMinifier, ExternalTransform, CopyRule) | ✅ Done |

### 🔲 Remaining (Phase 4 - Advanced)

| Step | Description | Status |
|------|-------------|--------|
| 8 | Graph Integration (topological sort) | 🔲 Partial |
| 9 | Image optimization transforms | 🔲 Not started |

---

## Core Concepts

### Build Phases

```
Discover → Transform → Generate → Finalize
    │          │           │          │
    │          │           │          └─ Sitemap, RSS, search index
    │          │           └─ MD→HTML, HTML→HTML, parametric expansion
    │          └─ SCSS→CSS, image optimization, bundling
    └─ Find all resources, identify types, load metadata
```

**Verified working output:**
```
2025/11/29 02:32:33 === Phase: Discover ===
2025/11/29 02:32:33 === Phase: Transform ===
2025/11/29 02:32:33 === Phase: Generate ===
2025/11/29 02:32:34 === Phase: Finalize ===
```

### Resource Model Enhancement

Resources now have awareness of their **relationships**:

```go
type Resource struct {
    // Existing fields...

    // New: Asset relationships
    Assets      []*Resource          // Co-located files (images, data)
    AssetOf     *Resource            // Parent resource if this is an asset

    // New: Build tracking
    ProducedBy  Rule                 // Which rule created this target
    ProducedAt  BuildPhase           // Which phase
}
```

### Rule Interface Evolution

```go
// Existing Rule interface (unchanged for backwards compat)
type Rule interface {
    TargetsFor(site *Site, res *Resource) (siblings []*Resource, targets []*Resource)
    Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error
}

// New: Optional interface for phase-aware rules
type PhaseRule interface {
    Rule
    Phase() BuildPhase
    DependsOn() []string   // glob patterns: "**/*.css"
    Produces() []string    // glob patterns: "**/*.min.css"
}

// New: Optional interface for asset-aware rules
type AssetAwareRule interface {
    Rule
    // HandleAssets is called with co-located assets for a resource
    // Returns which assets should be copied/processed and where
    HandleAssets(site *Site, res *Resource, assets []*Resource) ([]AssetMapping, error)
}

type AssetMapping struct {
    Source *Resource
    Dest   string      // relative to output
    Action AssetAction // Copy, Process, Skip
}

type AssetAction int
const (
    AssetCopy AssetAction = iota
    AssetProcess  // Run through transform rules
    AssetSkip
)
```

---

## Files Created/Modified

### New Files

| File | Purpose |
|------|---------|
| `phase.go` | BuildPhase, BuildContext, HookRegistry, PhaseRule, AssetAwareRule, LegacyRuleAdapter |
| `assets.go` | contentHash(), ContentHashShort(), GetAssetURL(), DefaultAssetHandler |
| `generators.go` | SitemapGenerator, RSSGenerator - hook-based Finalize phase generators |
| `transforms.go` | CSSMinifier, ExternalTransform, CopyRule - Transform phase rules |

### Modified Files

| File | Changes |
|------|---------|
| `resource.go` | Added Assets, AssetOf, ProducedBy, ProducedAt fields |
| `site.go` | Added PhaseRules, Hooks, AssetPatterns, SharedAssetsDir; Rewrote Rebuild() with 4-phase structure |
| `md.go` | Implemented PhaseRule interface (Phase=Generate) |
| `html.go` | Implemented PhaseRule interface (Phase=Generate) |
| `parampage.go` | Implemented PhaseRule interface (Phase=Generate) |

---

## Co-Located Assets Solution

### Problem Today

```
content/
├── blog/
│   └── my-post/
│       ├── index.md          → output/blog/my-post/index.html
│       ├── diagram.png       → ??? (copied to output/blog/my-post/diagram.png OR lost)
│       └── data.json         → ??? (might be processed as content)
```

Issues:
1. Images referenced as `/static/images/diagram.png` - disconnected from post
2. If copied, parametric pages duplicate assets unnecessarily
3. No way to distinguish "asset of post" vs "standalone resource"

### Solution: Explicit Asset Declaration

Assets are declared explicitly via site config (not auto-detected):

```go
site := s3.Site{
    AssetPatterns: []string{
        "*.png", "*.jpg", "*.jpeg", "*.gif", "*.svg", "*.webp",
        "*.mp4", "*.webm", "*.pdf",
    },
}
```

### Parametric Pages: Shared Assets Folder

For parametric pages like `[tag].html`, assets go to a shared location to avoid duplication:

**Output structure with shared assets:**
```
output/
├── _assets/                    # Shared assets folder
│   └── a1b2c3d4/              # Content-hash prefix
│       └── diagram.png         # Shared across parametric expansions
├── tags/
│   ├── go/index.html          # References /_assets/a1b2c3d4/diagram.png
│   ├── rust/index.html        # References same asset (no duplication)
│   └── python/index.html
└── blog/
    └── my-post/
        ├── index.html          # Non-parametric: assets co-located
        └── diagram.png         # Local copy (./diagram.png reference)
```

### Usage in Templates

```go
// Template function to get correct asset URL
{{ AssetURL "diagram.png" }}
// Returns "./diagram.png" for normal pages
// Returns "/_assets/a1b2c3d4/diagram.png" for parametric pages
```

---

## Backwards Compatibility

Legacy rules (those not implementing PhaseRule) are automatically wrapped with `LegacyRuleAdapter`:

```go
type LegacyRuleAdapter struct {
    Wrapped Rule
}

func (l *LegacyRuleAdapter) Phase() BuildPhase {
    return PhaseGenerate // Legacy rules run in Generate phase
}

func (l *LegacyRuleAdapter) DependsOn() []string { return nil }
func (l *LegacyRuleAdapter) Produces() []string { return nil }
```

This ensures existing sites continue to work without modification.

---

## Future Work (Phase 2)

### Example Generators

**SitemapGenerator:**
```go
type SitemapGenerator struct {
    BaseURL string
    urls    []SitemapURL
}

func (g *SitemapGenerator) Phase() BuildPhase {
    return PhaseFinalize
}

func (g *SitemapGenerator) DependsOn() []string {
    return []string{"**/*.html"} // Needs all HTML generated first
}

// Registration via hooks
func (g *SitemapGenerator) Register(site *Site) {
    site.Hooks.OnResourceProcessed(func(ctx *BuildContext, res *Resource, targets []*Resource) {
        // Collect URLs
    })

    site.Hooks.OnPhaseEnd(PhaseFinalize, func(ctx *BuildContext) {
        g.writeSitemap(ctx.Site.OutputDir)
    })
}
```

**RSSGenerator:** Similar pattern, collects posts and writes feed.xml

### Transform Rules

**SCSSToCSS:**
```go
type SCSSToCSS struct{}

func (s *SCSSToCSS) Phase() BuildPhase {
    return PhaseTransform
}

func (s *SCSSToCSS) DependsOn() []string {
    return []string{"**/*.scss"}
}

func (s *SCSSToCSS) Produces() []string {
    return []string{"**/*.css"}
}
```

### Graph Integration

Full topological sort for rule ordering based on DependsOn/Produces patterns:

```go
func (s *Site) topologicalSort(rules []Rule) []Rule {
    // Build dependency graph based on DependsOn/Produces overlap
    // Use Kahn's algorithm for cycle-free ordering
    // Fall back to declaration order if no dependencies
}
```

---

## Future Extensions (Not in This Plan)

- **Async processing**: Parallelize within phases
- **Incremental builds**: Track dependencies, rebuild only changed
- **Watch mode integration**: Re-run affected phases on file change
- **Plugin system**: Dynamic rule loading

---

## Testing

The implementation was tested with a blog site:

```bash
make run
# Output:
# === Phase: Discover ===
# === Phase: Transform ===
# === Phase: Generate ===
# INFO Discovering params for resource=.../content/tags/[tag].html
# INFO Discovered params ... values="[algorithms ... golang ...]"
# INFO Discovering params for resource=.../content/blog/page/[page].html
# INFO Discovered params ... values="[1 2]"
# === Phase: Finalize ===
```

**Verified:**
- Homepage renders correctly
- Blog posts render correctly
- Parametric pages (tags, pagination) work
- CSS loads correctly
- All HTTP 200 responses
