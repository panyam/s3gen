
# Phase-Based Architecture

`s3gen` uses a four-phase build pipeline that provides fine-grained control over how your site is built. This architecture enables transformation pipelines, side-effect generators, and proper ordering of build steps.

## The Four Phases

```
Discover → Transform → Generate → Finalize
```

### Phase 1: Discover

The Discover phase walks your content directory and prepares resources for processing.

**What happens:**
- Walks `ContentRoot` to find all files
- Creates a `Resource` object for each file
- Loads front matter metadata
- Identifies co-located assets based on `AssetPatterns`
- Sorts resources by `PriorityFunc`

**No rules run in this phase** - it's handled internally by the build system.

### Phase 2: Transform

The Transform phase processes source assets before content generation.

**Use cases:**
- SCSS → CSS compilation
- TypeScript → JavaScript compilation
- CSS minification
- Image optimization
- Any preprocessing step

**Example rules:** `CSSMinifier`, `ExternalTransform`, `CopyRule`

### Phase 3: Generate

The Generate phase produces final HTML output from your content.

**Use cases:**
- Markdown → HTML conversion (`MDToHtml`)
- HTML template processing (`HTMLToHtml`)
- Parametric page expansion (`ParametricPages`)

**This is where most content rules run.** Legacy rules that don't implement `PhaseRule` are automatically assigned to this phase.

### Phase 4: Finalize

The Finalize phase runs after all content is generated, with access to the complete list of generated files.

**Use cases:**
- Sitemap generation
- RSS feed generation
- Search index creation
- Any site-wide artifact

**Example rules:** `SitemapGenerator`, `RSSGenerator`

## Build Phases in Code

```go
type BuildPhase int

const (
    PhaseDiscover  BuildPhase = iota  // Find all resources
    PhaseTransform                     // Asset transformations
    PhaseGenerate                      // Content → HTML
    PhaseFinalize                      // Site-wide artifacts
)
```

## The PhaseRule Interface

To participate in the phase-based pipeline, rules implement the `PhaseRule` interface:

```go
type PhaseRule interface {
    Rule

    // Phase returns which build phase this rule runs in.
    Phase() BuildPhase

    // DependsOn returns glob patterns of files this rule needs as input.
    // Used for ordering rules within a phase.
    DependsOn() []string

    // Produces returns glob patterns of files this rule creates.
    // Used for ordering rules within a phase.
    Produces() []string
}
```

### Example: Transform Phase Rule

```go
type CSSMinifier struct {
    // ... fields
}

func (m *CSSMinifier) Phase() BuildPhase {
    return PhaseTransform
}

func (m *CSSMinifier) DependsOn() []string {
    return []string{"**/*.css"}
}

func (m *CSSMinifier) Produces() []string {
    return []string{"**/*.min.css"}
}
```

### Example: Finalize Phase Rule

```go
type SitemapGenerator struct {
    // ... fields
}

func (g *SitemapGenerator) Phase() BuildPhase {
    return PhaseFinalize
}

func (g *SitemapGenerator) DependsOn() []string {
    return []string{"**/*.html"}  // Need all HTML generated first
}

func (g *SitemapGenerator) Produces() []string {
    return []string{"sitemap.xml"}
}
```

## Rule Ordering Within Phases

Rules within a phase are ordered based on their `DependsOn()` and `Produces()` patterns:

1. Rules that produce what other rules depend on run first
2. Rules with no dependencies run in declaration order
3. Circular dependencies are detected and reported as errors

### Example Ordering

```go
// These rules will be automatically ordered correctly:

// 1. SCSSTransform runs first (produces CSS)
&ExternalTransform{
    Name:            "SCSS",
    SourceExtension: ".scss",
    TargetExtension: ".css",
}

// 2. CSSMinifier runs second (depends on CSS, produces .min.css)
&CSSMinifier{
    ExcludePatterns: []string{"*.min.css"},
}
```

## BuildContext

During a build, a `BuildContext` is passed through all phases:

```go
type BuildContext struct {
    Site             *Site
    CurrentPhase     BuildPhase
    Resources        []*Resource      // All discovered resources
    CreatedInPhase   map[BuildPhase][]*Resource  // Resources created per phase
    GeneratedTargets []*Resource      // All generated targets
    Errors           []error          // Non-fatal errors
    hooks            *HookRegistry    // For callbacks
}
```

Rules and hooks can use the context to:
- Access all discovered resources
- See what targets have been generated
- Add non-fatal errors
- Track phase-specific state

## Legacy Rule Compatibility

Rules that don't implement `PhaseRule` are automatically wrapped:

```go
type LegacyRuleAdapter struct {
    Wrapped Rule
}

func (l *LegacyRuleAdapter) Phase() BuildPhase {
    return PhaseGenerate  // Legacy rules run in Generate phase
}

func (l *LegacyRuleAdapter) DependsOn() []string { return nil }
func (l *LegacyRuleAdapter) Produces() []string { return nil }
```

This ensures existing sites continue to work without modification.

## Registering Phase Rules

Rules are automatically categorized by phase when added to `BuildRules`:

```go
var Site = s3.Site{
    BuildRules: []s3.Rule{
        // Transform phase rules
        &s3.CSSMinifier{...},
        s3.NewSCSSTransform(),

        // Generate phase rules (or legacy rules)
        &s3.MDToHtml{...},
        &s3.HTMLToHtml{...},
    },
}
```

The site organizes them internally:

```go
// During initialization:
site.PhaseRules[PhaseTransform] = []*CSSMinifier, *ExternalTransform, ...
site.PhaseRules[PhaseGenerate]  = []*MDToHtml, *HTMLToHtml, ...
site.PhaseRules[PhaseFinalize]  = []*SitemapGenerator, *RSSGenerator, ...
```

## Build Output

During a build, you'll see phase transitions in the logs:

```
2025/11/29 02:32:33 === Phase: Discover ===
2025/11/29 02:32:33 === Phase: Transform ===
2025/11/29 02:32:33 === Phase: Generate ===
2025/11/29 02:32:34 INFO Discovering params for resource=.../[tag].html
2025/11/29 02:32:34 INFO Discovered params values="[go rust python ...]"
2025/11/29 02:32:34 === Phase: Finalize ===
```

## Best Practices

1. **Choose the right phase:**
   - Use `PhaseTransform` for asset processing
   - Use `PhaseGenerate` for content → HTML conversion
   - Use `PhaseFinalize` for site-wide artifacts

2. **Declare dependencies accurately:**
   - Use `DependsOn()` to specify what files your rule needs
   - Use `Produces()` to specify what files your rule creates
   - This enables proper ordering

3. **Use hooks for Finalize:**
   - Finalize rules often use hooks to collect data during the build
   - See [Generators and Hooks](09-generators-and-hooks.md) for details

4. **Test with verbose logging:**
   - Enable `slog.SetLogLoggerLevel(slog.LevelDebug)` to see detailed phase info
