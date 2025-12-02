
# Transform Rules

Transform rules process source files during the Transform phase, before content generation. They're used for asset compilation, minification, and preprocessing.

## Built-in Transform Rules

### CSSMinifier

Minifies CSS files, optionally using an external tool.

```go
&s3.CSSMinifier{
    // Command for external minifier (optional)
    // If empty, uses built-in simple minifier
    Command: "lightningcss",

    // Arguments for the command (input piped via stdin)
    Args: []string{"--minify"},

    // Suffix added before .css extension
    // e.g., ".min" produces file.min.css
    OutputSuffix: ".min",

    // Glob patterns to match (default: ["**/*.css"])
    SourcePatterns: []string{"static/**/*.css"},

    // Glob patterns to exclude
    ExcludePatterns: []string{"*.min.css", "vendor/**"},
}
```

**Usage:**

```go
var Site = s3.Site{
    BuildRules: []s3.Rule{
        // Create .min.css versions of all CSS files
        &s3.CSSMinifier{
            OutputSuffix:    ".min",
            ExcludePatterns: []string{"*.min.css"},
        },

        // ... other rules
    },
}
```

### ExternalTransform

Runs any external command to transform files. Supports both file-based and stdin/stdout modes.

```go
&s3.ExternalTransform{
    // Descriptive name for logging
    Name: "SCSS",

    // Command to run
    Command: "sass",

    // Arguments - use {input} and {output} placeholders
    Args: []string{"--no-source-map", "{input}", "{output}"},

    // Source file extension to match
    SourceExtension: ".scss",

    // Output file extension
    TargetExtension: ".css",

    // Additional glob patterns (optional)
    SourcePatterns: []string{"styles/**/*.scss"},

    // Patterns to exclude
    ExcludePatterns: []string{"_*.scss"},

    // Working directory for command (optional)
    WorkingDir: "./styles",
}
```

**File placeholders:**
- `{input}` - replaced with input file path
- `{output}` - replaced with output file path
- If neither is used, input is piped via stdin and output read from stdout

### CopyRule

Copies files matching patterns from content to output without transformation.

```go
&s3.CopyRule{
    // Glob patterns to match
    Patterns: []string{"*.woff", "*.woff2", "*.ttf", "*.eot"},

    // Patterns to exclude
    ExcludePatterns: []string{"test-*"},

    // Optional: flatten all files into a single directory
    FlattenDir: "fonts",
}
```

## Convenience Functions

`s3gen` provides helper functions for common transforms:

### NewSCSSTransform

Creates an ExternalTransform for SCSS compilation using `sass`:

```go
// Requires sass or dart-sass to be installed
scssRule := s3.NewSCSSTransform()

// Equivalent to:
&s3.ExternalTransform{
    Name:            "SCSS",
    Command:         "sass",
    Args:            []string{"--no-source-map", "{input}", "{output}"},
    SourceExtension: ".scss",
    TargetExtension: ".css",
}
```

### NewTypeScriptTransform

Creates an ExternalTransform for TypeScript compilation using `esbuild`:

```go
// Requires esbuild to be installed
tsRule := s3.NewTypeScriptTransform()

// Equivalent to:
&s3.ExternalTransform{
    Name:            "TypeScript",
    Command:         "esbuild",
    Args:            []string{"--bundle", "--outfile={output}", "{input}"},
    SourceExtension: ".ts",
    TargetExtension: ".js",
}
```

### NewTailwindTransform

Creates an ExternalTransform for Tailwind CSS processing:

```go
// Requires tailwindcss CLI to be installed
tailwindRule := s3.NewTailwindTransform("src/input.css", "static/output.css")

// Equivalent to:
&s3.ExternalTransform{
    Name:    "Tailwind",
    Command: "tailwindcss",
    Args:    []string{"-i", "src/input.css", "-o", "static/output.css", "--minify"},
}
```

## Creating Custom Transform Rules

### Basic Structure

```go
type MyTransform struct {
    // Configuration fields
}

// Declare this runs in Transform phase
func (t *MyTransform) Phase() s3.BuildPhase {
    return s3.PhaseTransform
}

// Declare input dependencies
func (t *MyTransform) DependsOn() []string {
    return []string{"**/*.less"}
}

// Declare output files
func (t *MyTransform) Produces() []string {
    return []string{"**/*.css"}
}

// Check if rule applies and define output
func (t *MyTransform) TargetsFor(site *s3.Site, res *s3.Resource) ([]*s3.Resource, []*s3.Resource) {
    if !strings.HasSuffix(res.FullPath, ".less") {
        return nil, nil
    }

    // Calculate output path
    outPath := strings.TrimSuffix(res.FullPath, ".less") + ".css"
    relPath, _ := filepath.Rel(site.ContentRoot, outPath)
    destPath := filepath.Join(site.OutputDir, relPath)

    target := site.GetResource(destPath)
    target.Source = res

    return []*s3.Resource{res}, []*s3.Resource{target}
}

// Perform the transformation
func (t *MyTransform) Run(site *s3.Site, inputs []*s3.Resource, targets []*s3.Resource, funcs map[string]any) error {
    input := inputs[0]
    output := targets[0]
    output.EnsureDir()

    // Your transformation logic here
    data, err := input.ReadAll()
    if err != nil {
        return err
    }

    transformed := processLess(data)
    return os.WriteFile(output.FullPath, transformed, 0644)
}
```

### Using External Commands

```go
func (t *MyTransform) Run(site *s3.Site, inputs []*s3.Resource, targets []*s3.Resource, funcs map[string]any) error {
    input := inputs[0]
    output := targets[0]
    output.EnsureDir()

    cmd := exec.Command("lessc", input.FullPath, output.FullPath)
    var stderr bytes.Buffer
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("lessc failed: %s", stderr.String())
    }

    return nil
}
```

### Piping Through Commands

```go
func (t *MyTransform) Run(site *s3.Site, inputs []*s3.Resource, targets []*s3.Resource, funcs map[string]any) error {
    input := inputs[0]
    output := targets[0]
    output.EnsureDir()

    // Read input
    data, err := input.ReadAll()
    if err != nil {
        return err
    }

    // Pipe through command
    cmd := exec.Command("uglifyjs")
    cmd.Stdin = bytes.NewReader(data)

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("uglifyjs failed: %s", stderr.String())
    }

    return os.WriteFile(output.FullPath, stdout.Bytes(), 0644)
}
```

## Transform Pipelines

Transform rules are ordered based on `DependsOn()` and `Produces()` patterns, enabling pipelines:

```go
var Site = s3.Site{
    BuildRules: []s3.Rule{
        // 1. SCSS → CSS (runs first - produces CSS)
        s3.NewSCSSTransform(),

        // 2. CSS Minification (runs second - depends on CSS)
        &s3.CSSMinifier{
            OutputSuffix:    ".min",
            ExcludePatterns: []string{"*.min.css"},
        },

        // ... content rules
    },
}
```

The build system automatically orders these correctly:
1. `NewSCSSTransform` produces `**/*.css`
2. `CSSMinifier` depends on `**/*.css`
3. Therefore SCSS compilation runs before minification

## Best Practices

1. **Use ExternalTransform for simple cases**: Don't reinvent the wheel if an external tool exists

2. **Declare accurate dependencies**: This enables proper ordering

3. **Handle errors gracefully**: Return informative error messages

4. **Log progress**: Use `log.Printf` for visibility during development

5. **Test with clean builds**: Remove output directory to verify transforms work from scratch

6. **Exclude generated files**: Use `ExcludePatterns` to prevent double-processing (e.g., `*.min.css`)

## Example: Complete Build Pipeline

```go
var Site = s3.Site{
    ContentRoot: "./content",
    OutputDir:   "./public",

    BuildRules: []s3.Rule{
        // === Transform Phase ===

        // Compile SCSS
        s3.NewSCSSTransform(),

        // Minify CSS
        &s3.CSSMinifier{
            OutputSuffix:    ".min",
            ExcludePatterns: []string{"*.min.css"},
        },

        // Compile TypeScript
        s3.NewTypeScriptTransform(),

        // Copy fonts
        &s3.CopyRule{
            Patterns: []string{"*.woff2", "*.woff", "*.ttf"},
        },

        // === Generate Phase ===

        // Handle parametric pages first
        &s3.ParametricPages{
            Renderers: map[string]s3.Rule{
                ".md":   &s3.MDToHtml{...},
                ".html": &s3.HTMLToHtml{...},
            },
        },

        // Standard content
        &s3.MDToHtml{...},
        &s3.HTMLToHtml{...},
    },
}
```
