package s3gen

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CSSMinifier minifies CSS files during the Transform phase.
// It can use external tools (like lightningcss, csso, or clean-css) or a simple built-in minifier.
type CSSMinifier struct {
	// Command is the external minifier command (e.g., "lightningcss", "csso", "cleancss")
	// If empty, uses a simple built-in minifier
	Command string

	// Args are additional arguments to pass to the command
	// The input file will be appended as the last argument
	Args []string

	// OutputSuffix is appended before .css extension (e.g., ".min" produces file.min.css)
	// If empty, the file is minified in-place (same name)
	OutputSuffix string

	// SourcePatterns are glob patterns to match CSS files (default: ["**/*.css"])
	SourcePatterns []string

	// ExcludePatterns are glob patterns to exclude (e.g., ["**/*.min.css"])
	ExcludePatterns []string
}

func (m *CSSMinifier) Phase() BuildPhase {
	return PhaseTransform
}

func (m *CSSMinifier) DependsOn() []string {
	if len(m.SourcePatterns) > 0 {
		return m.SourcePatterns
	}
	return []string{"**/*.css"}
}

func (m *CSSMinifier) Produces() []string {
	if m.OutputSuffix != "" {
		return []string{"**/*" + m.OutputSuffix + ".css"}
	}
	return []string{"**/*.css"}
}

func (m *CSSMinifier) TargetsFor(site *Site, res *Resource) ([]*Resource, []*Resource) {
	if !strings.HasSuffix(res.FullPath, ".css") {
		return nil, nil
	}

	// Check exclusions
	relPath := res.RelPath()
	for _, pattern := range m.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return nil, nil
		}
		// Also check just the filename
		if matched, _ := filepath.Match(pattern, filepath.Base(res.FullPath)); matched {
			return nil, nil
		}
	}

	// Determine output path
	var outPath string
	if m.OutputSuffix != "" {
		ext := filepath.Ext(res.FullPath)
		base := res.FullPath[:len(res.FullPath)-len(ext)]
		outPath = base + m.OutputSuffix + ext
	} else {
		outPath = res.FullPath
	}

	// Map to output directory
	outRelPath, _ := filepath.Rel(site.ContentRoot, outPath)
	destPath := filepath.Join(site.OutputDir, outRelPath)

	target := site.GetResource(destPath)
	target.Source = res
	return []*Resource{res}, []*Resource{target}
}

func (m *CSSMinifier) Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error {
	if len(inputs) != 1 || len(targets) != 1 {
		return fmt.Errorf("CSSMinifier: expected 1 input and 1 output, got %d and %d", len(inputs), len(targets))
	}

	input := inputs[0]
	output := targets[0]
	output.EnsureDir()

	// Read input
	data, err := input.ReadAll()
	if err != nil {
		return fmt.Errorf("CSSMinifier: failed to read %s: %w", input.FullPath, err)
	}

	var minified []byte

	if m.Command != "" {
		// Use external command
		minified, err = m.runExternal(data)
		if err != nil {
			return fmt.Errorf("CSSMinifier: external command failed: %w", err)
		}
	} else {
		// Use simple built-in minifier
		minified = m.minifySimple(data)
	}

	return os.WriteFile(output.FullPath, minified, 0644)
}

func (m *CSSMinifier) runExternal(input []byte) ([]byte, error) {
	cmd := exec.Command(m.Command, m.Args...)
	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// minifySimple is a basic CSS minifier that removes comments and unnecessary whitespace.
func (m *CSSMinifier) minifySimple(input []byte) []byte {
	var buf bytes.Buffer
	inComment := false
	inString := false
	stringChar := byte(0)
	lastChar := byte(0)

	for i := 0; i < len(input); i++ {
		c := input[i]

		// Handle strings
		if inString {
			buf.WriteByte(c)
			if c == stringChar && lastChar != '\\' {
				inString = false
			}
			lastChar = c
			continue
		}

		// Check for string start
		if c == '"' || c == '\'' {
			inString = true
			stringChar = c
			buf.WriteByte(c)
			lastChar = c
			continue
		}

		// Handle comments
		if !inComment && c == '/' && i+1 < len(input) && input[i+1] == '*' {
			inComment = true
			i++
			continue
		}
		if inComment {
			if c == '*' && i+1 < len(input) && input[i+1] == '/' {
				inComment = false
				i++
			}
			continue
		}

		// Collapse whitespace
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			// Only keep space if needed (after alphanumeric, before alphanumeric)
			if buf.Len() > 0 {
				last := buf.Bytes()[buf.Len()-1]
				// Skip space after these characters
				if last == '{' || last == '}' || last == ':' || last == ';' || last == ',' || last == '>' || last == '+' || last == '~' {
					continue
				}
			}
			// Look ahead to see if space is needed
			for i+1 < len(input) {
				next := input[i+1]
				if next == ' ' || next == '\t' || next == '\n' || next == '\r' {
					i++
					continue
				}
				// Skip space before these characters
				if next == '{' || next == '}' || next == ':' || next == ';' || next == ',' || next == '>' || next == '+' || next == '~' {
					break
				}
				// Need a space
				buf.WriteByte(' ')
				break
			}
			continue
		}

		buf.WriteByte(c)
		lastChar = c
	}

	return buf.Bytes()
}

// ExternalTransform runs an external command to transform files.
// This is a generic transform rule that can be used for any command-line tool.
type ExternalTransform struct {
	// Name is a descriptive name for this transform (for logging)
	Name string

	// Command is the command to run
	Command string

	// Args are arguments to the command.
	// Special placeholders:
	//   {input}  - replaced with input file path
	//   {output} - replaced with output file path
	// If neither placeholder is used, input is piped via stdin and output read from stdout.
	Args []string

	// SourceExtension is the file extension to match (e.g., ".scss", ".ts")
	SourceExtension string

	// TargetExtension is the output file extension (e.g., ".css", ".js")
	TargetExtension string

	// SourcePatterns are additional glob patterns to match (optional)
	SourcePatterns []string

	// ExcludePatterns are glob patterns to exclude
	ExcludePatterns []string

	// WorkingDir is the working directory for the command (default: site root)
	WorkingDir string
}

func (t *ExternalTransform) Phase() BuildPhase {
	return PhaseTransform
}

func (t *ExternalTransform) DependsOn() []string {
	if len(t.SourcePatterns) > 0 {
		return t.SourcePatterns
	}
	if t.SourceExtension != "" {
		return []string{"**/*" + t.SourceExtension}
	}
	return nil
}

func (t *ExternalTransform) Produces() []string {
	if t.TargetExtension != "" {
		return []string{"**/*" + t.TargetExtension}
	}
	return nil
}

func (t *ExternalTransform) TargetsFor(site *Site, res *Resource) ([]*Resource, []*Resource) {
	// Check extension match
	if t.SourceExtension != "" && !strings.HasSuffix(res.FullPath, t.SourceExtension) {
		return nil, nil
	}

	// Check exclusions
	relPath := res.RelPath()
	for _, pattern := range t.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return nil, nil
		}
	}

	// Determine output path
	outPath := res.FullPath
	if t.SourceExtension != "" && t.TargetExtension != "" {
		outPath = strings.TrimSuffix(outPath, t.SourceExtension) + t.TargetExtension
	}

	// Map to output directory
	outRelPath, _ := filepath.Rel(site.ContentRoot, outPath)
	destPath := filepath.Join(site.OutputDir, outRelPath)

	target := site.GetResource(destPath)
	target.Source = res
	return []*Resource{res}, []*Resource{target}
}

func (t *ExternalTransform) Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error {
	if len(inputs) != 1 || len(targets) != 1 {
		return fmt.Errorf("%s: expected 1 input and 1 output, got %d and %d", t.Name, len(inputs), len(targets))
	}

	input := inputs[0]
	output := targets[0]
	output.EnsureDir()

	// Check if using file placeholders
	useFiles := false
	args := make([]string, len(t.Args))
	for i, arg := range t.Args {
		if strings.Contains(arg, "{input}") || strings.Contains(arg, "{output}") {
			useFiles = true
		}
		args[i] = strings.ReplaceAll(arg, "{input}", input.FullPath)
		args[i] = strings.ReplaceAll(args[i], "{output}", output.FullPath)
	}

	cmd := exec.Command(t.Command, args...)

	if t.WorkingDir != "" {
		cmd.Dir = t.WorkingDir
	}

	if useFiles {
		// Command handles files directly
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %s: %s", t.Name, err, stderr.String())
		}
	} else {
		// Pipe stdin/stdout
		data, err := input.ReadAll()
		if err != nil {
			return fmt.Errorf("%s: failed to read %s: %w", t.Name, input.FullPath, err)
		}

		cmd.Stdin = bytes.NewReader(data)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %s: %s", t.Name, err, stderr.String())
		}

		if err := os.WriteFile(output.FullPath, stdout.Bytes(), 0644); err != nil {
			return fmt.Errorf("%s: failed to write %s: %w", t.Name, output.FullPath, err)
		}
	}

	log.Printf("[%s] %s -> %s", t.Name, input.FullPath, output.FullPath)
	return nil
}

// CopyRule copies files matching patterns from content to output.
// Useful for static assets that don't need transformation.
type CopyRule struct {
	// Patterns are glob patterns to match files
	Patterns []string

	// ExcludePatterns are glob patterns to exclude
	ExcludePatterns []string

	// FlattenDir if true, copies all files to a single directory
	FlattenDir string
}

func (c *CopyRule) Phase() BuildPhase {
	return PhaseTransform
}

func (c *CopyRule) DependsOn() []string {
	return c.Patterns
}

func (c *CopyRule) Produces() []string {
	return c.Patterns
}

func (c *CopyRule) TargetsFor(site *Site, res *Resource) ([]*Resource, []*Resource) {
	relPath := res.RelPath()

	// Check if matches any pattern
	matched := false
	for _, pattern := range c.Patterns {
		if m, _ := filepath.Match(pattern, relPath); m {
			matched = true
			break
		}
		if m, _ := filepath.Match(pattern, filepath.Base(relPath)); m {
			matched = true
			break
		}
	}
	if !matched {
		return nil, nil
	}

	// Check exclusions
	for _, pattern := range c.ExcludePatterns {
		if m, _ := filepath.Match(pattern, relPath); m {
			return nil, nil
		}
	}

	// Determine output path
	var destPath string
	if c.FlattenDir != "" {
		destPath = filepath.Join(site.OutputDir, c.FlattenDir, filepath.Base(res.FullPath))
	} else {
		destPath = filepath.Join(site.OutputDir, relPath)
	}

	target := site.GetResource(destPath)
	target.Source = res
	return []*Resource{res}, []*Resource{target}
}

func (c *CopyRule) Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error {
	if len(inputs) != 1 || len(targets) != 1 {
		return fmt.Errorf("CopyRule: expected 1 input and 1 output, got %d and %d", len(inputs), len(targets))
	}

	input := inputs[0]
	output := targets[0]
	output.EnsureDir()

	// Copy file
	src, err := os.Open(input.FullPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(output.FullPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// Convenience functions for common transforms

// NewSCSSTransform creates an ExternalTransform for SCSS to CSS compilation.
// Requires `sass` or `dart-sass` to be installed.
func NewSCSSTransform() *ExternalTransform {
	return &ExternalTransform{
		Name:            "SCSS",
		Command:         "sass",
		Args:            []string{"--no-source-map", "{input}", "{output}"},
		SourceExtension: ".scss",
		TargetExtension: ".css",
	}
}

// NewTypeScriptTransform creates an ExternalTransform for TypeScript compilation.
// Requires `tsc` or `esbuild` to be installed.
func NewTypeScriptTransform() *ExternalTransform {
	return &ExternalTransform{
		Name:            "TypeScript",
		Command:         "esbuild",
		Args:            []string{"--bundle", "--outfile={output}", "{input}"},
		SourceExtension: ".ts",
		TargetExtension: ".js",
	}
}

// NewTailwindTransform creates an ExternalTransform for Tailwind CSS.
// Requires `tailwindcss` CLI to be installed.
func NewTailwindTransform(inputFile, outputFile string) *ExternalTransform {
	return &ExternalTransform{
		Name:    "Tailwind",
		Command: "tailwindcss",
		Args:    []string{"-i", inputFile, "-o", outputFile, "--minify"},
	}
}
