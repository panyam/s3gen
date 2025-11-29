// Package s3gen is a simple, flexible, rule-based static site generator for Go developers.
package s3gen

import (
	"bytes"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/mux"
	gotl "github.com/panyam/goutils/template"
	gut "github.com/panyam/goutils/utils"
	tmplr "github.com/panyam/templar"
	"github.com/radovskyb/watcher"
)

// Site is the central object in s3gen. It contains all the configuration
// and metadata for the website, and it orchestrates the build process.
type Site struct {
	// Templates is the template group that holds all the parsed templates
	// and their functions.
	Templates *tmplr.TemplateGroup

	// CommonFuncMap is a map of functions that will be available in all templates.
	CommonFuncMap map[string]any

	// TemplateFolders is a list of directories where s3gen will look for templates.
	TemplateFolders []string

	// LoaderList is the list of template loaders.
	LoaderList *tmplr.LoaderList

	// ContentRoot is the root directory of your website's content. s3gen will
	// walk this directory to find all the files to process.
	ContentRoot string

	// OutputDir is the directory where the generated static files will be written.
	OutputDir string

	// PathPrefix is the URL path prefix for the site. For example, if your site
	// is served at mydomain.com/blog, your PathPrefix would be "/blog".
	PathPrefix string

	// StaticFolders is a list of directories that will be served as-is. It's
	// a slice of strings in the format [path1, dir1, path2, dir2, ...].
	StaticFolders []string

	// IgnoreDirFunc is a function that determines whether a directory should be
	// ignored during the build process.
	IgnoreDirFunc func(dirpath string) bool

	// IgnoreFileFunc is a function that determines whether a file should be
	// ignored during the build process.
	IgnoreFileFunc func(filepath string) bool

	// PriorityFunc is a function that determines the order in which resources
	// are processed. This is crucial for handling dependencies.
	PriorityFunc func(res *Resource) int

	// LiveReload enables or disables live reloading during development.
	LiveReload bool

	// LazyLoad enables or disables lazy loading of resources.
	LazyLoad bool

	// DefaultBaseTemplate is the default template to use for rendering pages.
	DefaultBaseTemplate BaseTemplate

	// GetTemplate is a function that can be used to override the default
	// template for a specific resource.
	GetTemplate func(res *Resource, out *BaseTemplate)

	// CreateResourceBase is a function that creates the base data structure
	// for a resource.
	CreateResourceBase func(res *Resource)

	// BuildRules is a list of rules that define how to process different
	// types of files.
	BuildRules []Rule

	// DefaultRule is the rule that will be used if no other rule matches a
	// resource.
	DefaultRule    Rule
	resourceInRule map[string]map[Rule]bool

	// BuildFrequency is the interval at which the site will be rebuilt when
	// in watch mode.
	BuildFrequency time.Duration

	// mux is the HTTP request multiplexer used for serving the site.
	mux *http.ServeMux

	// reloadWatcher is the file watcher used for live reloading.
	reloadWatcher *watcher.Watcher

	// resources is a map of all the resources in the site, keyed by their
	// full path.
	resources map[string]*Resource
	resedges  map[string][]string

	initialized bool

	// AssetPatterns defines glob patterns for files that should be treated
	// as assets of co-located content files. Patterns are relative to the
	// content file's directory. Example: []string{"*.png", "*.jpg", "*.svg"}
	AssetPatterns []string

	// PhaseRules organizes rules by the phase they run in.
	// This is populated during Init() from BuildRules.
	PhaseRules map[BuildPhase][]Rule

	// Hooks provides callbacks for observing build events.
	Hooks *HookRegistry

	// SharedAssetsDir is the directory name for shared assets (used by parametric pages).
	// Defaults to "_assets" if not set.
	SharedAssetsDir string
}

// Init initializes the Site object with default values.
func (s *Site) Init() *Site {
	s.ContentRoot = gut.ExpandUserPath(s.ContentRoot)
	s.resourceInRule = map[string]map[Rule]bool{}
	if len(s.BuildRules) == 0 {
		// setup some defaults
		s.BuildRules = []Rule{
			// A single, powerful rule for all parametric pages.
			&ParametricPages{
				Renderers: map[string]Rule{
					// It knows to use MDToHtml for .md and .mdx files...
					".md":  &MDToHtml{BaseToHtmlRule: BaseToHtmlRule{Extensions: []string{".md"}}},
					".mdx": &MDToHtml{BaseToHtmlRule: BaseToHtmlRule{Extensions: []string{".mdx"}}},
					// ...and HTMLToHtml for .html and .htm files.
					".html": &HTMLToHtml{BaseToHtmlRule: BaseToHtmlRule{Extensions: []string{".html"}}},
					".htm":  &HTMLToHtml{BaseToHtmlRule: BaseToHtmlRule{Extensions: []string{".htm"}}},
				},
			},
			&MDToHtml{BaseToHtmlRule: BaseToHtmlRule{Extensions: []string{".md", ".mdx"}}},
			&HTMLToHtml{BaseToHtmlRule: BaseToHtmlRule{Extensions: []string{".htm", ".html"}}},
		}
	}
	if s.PriorityFunc == nil {
		// use a default
		s.PriorityFunc = func(r *Resource) int {
			base := filepath.Base(r.FullPath)
			if base[0] == '[' && base[len(base)-1] == ']' {
				// parametric pages
				return 10000
			}
			if strings.HasPrefix(base, "index") || strings.HasPrefix(base, "_index") {
				// index pages
				return 5000
			}
			if strings.HasSuffix(base, ".md") || strings.HasSuffix(base, ".mdx") || strings.HasSuffix(base, ".html") || strings.HasSuffix(base, ".htm") {
				return 1000
			}
			return 0
		}
	}
	if s.Templates == nil {
		s.Templates = tmplr.NewTemplateGroup()
		s.LoaderList = &tmplr.LoaderList{}
		// Default loader is for templates
		s.LoaderList.DefaultLoader = tmplr.NewFileSystemLoader(s.TemplateFolders...)
		// s.LoaderList.AddLoader(&ContentLoader{s.ContentRoot})
		s.Templates.Loader = s.LoaderList
		s.Templates.AddFuncs(gotl.DefaultFuncMap())
		s.Templates.AddFuncs(s.DefaultFuncMap())
		s.Templates.AddFuncs(s.CommonFuncMap)
	}
	s.OutputDir = gut.ExpandUserPath(s.OutputDir)
	if s.CreateResourceBase == nil {
		s.CreateResourceBase = func(res *Resource) {
			res.Base = &DefaultResourceBase{Res: res}
			if err := res.Base.LoadFrom(res); err != nil {
				log.Println("error loading page: ", err)
			}
		}
	}
	if s.resources == nil {
		s.resources = make(map[string]*Resource)
	}

	// Initialize hooks
	if s.Hooks == nil {
		s.Hooks = NewHookRegistry()
	}

	// Set default shared assets directory
	if s.SharedAssetsDir == "" {
		s.SharedAssetsDir = "_assets"
	}

	// Migrate BuildRules to PhaseRules
	if s.PhaseRules == nil {
		s.PhaseRules = make(map[BuildPhase][]Rule)
	}
	for _, rule := range s.BuildRules {
		if phaseRule, ok := rule.(PhaseRule); ok {
			phase := phaseRule.Phase()
			s.PhaseRules[phase] = append(s.PhaseRules[phase], rule)
		} else {
			// Wrap legacy rule - it will run in Generate phase
			s.PhaseRules[PhaseGenerate] = append(s.PhaseRules[PhaseGenerate],
				&LegacyRuleAdapter{Wrapped: rule})
		}
	}

	s.initialized = true
	return s
}

// PathRelUrl returns the full URL for a path relative to the site's path prefix.
func (s *Site) PathRelUrl(path string) string {
	if s.PathPrefix == "" || s.PathPrefix == "/" {
		return path
	}
	return s.PathPrefix + path
}

// HandleStatic adds a new static path to the site's router.
func (s *Site) HandleStatic(path, folder string) *Site {
	s.StaticFolders = append(s.StaticFolders, path)
	s.StaticFolders = append(s.StaticFolders, folder)
	return s
}

// Handler returns an http.Handler that can be used to serve the site.
func (s *Site) Handler() http.Handler {
	if s.mux == nil {
		s.mux = http.NewServeMux()

		// Setup local/static paths
		for i := 0; i < len(s.StaticFolders); i += 2 {
			path, folder := s.StaticFolders[i], s.StaticFolders[i+1]
			log.Printf("Adding static route: %s -> %s", path, folder)
			s.mux.Handle(path, http.StripPrefix(path, http.FileServer(http.Dir(folder))))
			// s.filesRouter.PathPrefix(path).Handler(http.StripPrefix(path, http.FileServer(http.Dir(folder))))
		}

		// Serve everything else from the

		// Now add the file loader/handler for the "published" dir
		s.mux.Handle("/", http.FileServer(http.Dir(s.OutputDir)))
	}
	return s.mux
}

// ServeHTTP implements the http.Handler interface.
func (s *Site) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The entry point router for our site
	// parts := strings.Split(r.URL.Path, "/")[1:]
	// log.Println("1112 - URL Parts: ", parts)
	s.Handler().ServeHTTP(w, r)
}

// ListResources returns a list of resources in the site, with optional filtering and sorting.
func (s *Site) ListResources(filterFunc ResourceFilterFunc,
	sortFunc ResourceSortFunc,
	offset int, count int) (foundResources []*Resource) {
	// keep a map of files encountered and their statuses
	err := filepath.WalkDir(s.ContentRoot, func(fullpath string, info os.DirEntry, err error) error {
		if err != nil {
			// just print err related to the path and stop scanning
			// if this err means something else we can do other things here
			log.Println("Error in path: ", info, err)
			return err
		}

		if info.IsDir() {
			// see if we should this directory
			// eg if this is a special dir we may give it a different treatment
			// and return a SkipDir
			if s.IgnoreDirFunc != nil && s.IgnoreDirFunc(fullpath) {
				return filepath.SkipDir
			}
			return nil
		}

		if filterFunc == nil && s.IgnoreFileFunc != nil {
			if s.IgnoreFileFunc(fullpath) {
				return nil
			}
		}

		// map fullpath to a resource here
		res := s.GetResource(fullpath)

		if filterFunc == nil || filterFunc(res) {
			foundResources = append(foundResources, res)
		}

		return nil
	})

	if sortFunc != nil {
		sort.Slice(foundResources, func(idx1, idx2 int) bool {
			ent1 := foundResources[idx1]
			ent2 := foundResources[idx2]
			return sortFunc(ent1, ent2)
		})
	}
	if offset > 0 {
		foundResources = foundResources[offset:]
	}
	if count > 0 {
		if count > len(foundResources) {
			count = len(foundResources)
		}
		foundResources = foundResources[:count]
	}
	if err != nil {
		slog.Warn("Error walking dir: ", "error", err)
	}
	return
}

// GenerateSitemap generates a sitemap for the site.
func (s *Site) GenerateSitemap() map[string]any {
	return nil
}

// LoadParamValues loads the parameter values for a parametric resource.
func (s *Site) LoadParamValues(res *Resource) (err error) {
	if res.IsParametric {
		output := bytes.NewBufferString("")
		if res.ParamName != "" {
			panic("param name should have been empty")
		}
		log.Println("Rendering param values: ", res.FullPath)
		err = res.Renderer.Render(res, output)
		if err != nil {
			log.Println("Error executing paramvals template: ", err, res.FullPath)
		} else {
			log.Println("Param Values After: ", res.ParamValues, output)
		}
		slog.Info("Resource Is Parametric: ", "filepath", res.FullPath, "paramvalues", res.ParamValues, "err", err)
		if err != nil {
			log.Println("Error loading param values: ", res.FullPath, err)
		}
	}
	return
}

// Rebuild rebuilds the entire site using a 4-phase pipeline.
// If a list of resources is provided, only those resources will be rebuilt.
// Phases: Discover → Transform → Generate → Finalize
func (s *Site) Rebuild(rs []*Resource) {
	if !s.initialized {
		s.Init()
	}

	// Create build context
	ctx := &BuildContext{
		Site:           s,
		CreatedInPhase: make(map[BuildPhase][]*Resource),
		hooks:          s.Hooks,
	}

	// === PHASE: Discover ===
	ctx.CurrentPhase = PhaseDiscover
	log.Printf("=== Phase: %s ===", ctx.CurrentPhase)
	ctx.hooks.emitPhaseStart(ctx)

	if rs == nil {
		rs = s.ListResources(nil, nil, 0, 0)
	}

	// Discover assets for each content resource
	for _, res := range rs {
		s.discoverAssets(res)
	}

	// Sort by priority
	if s.PriorityFunc != nil {
		sort.Slice(rs, func(idx1, idx2 int) bool {
			return s.PriorityFunc(rs[idx1]) < s.PriorityFunc(rs[idx2])
		})
	}
	ctx.Resources = rs
	ctx.hooks.emitPhaseEnd(ctx)

	// === PHASE: Transform ===
	ctx.CurrentPhase = PhaseTransform
	log.Printf("=== Phase: %s ===", ctx.CurrentPhase)
	ctx.hooks.emitPhaseStart(ctx)
	s.runPhase(ctx, PhaseTransform)
	ctx.hooks.emitPhaseEnd(ctx)

	// === PHASE: Generate ===
	ctx.CurrentPhase = PhaseGenerate
	log.Printf("=== Phase: %s ===", ctx.CurrentPhase)
	ctx.hooks.emitPhaseStart(ctx)
	s.runPhase(ctx, PhaseGenerate)
	ctx.hooks.emitPhaseEnd(ctx)

	// Handle resources that didn't match any rule (default behavior)
	s.handleUnmatchedResources(ctx)

	// === PHASE: Finalize ===
	ctx.CurrentPhase = PhaseFinalize
	log.Printf("=== Phase: %s ===", ctx.CurrentPhase)
	ctx.hooks.emitPhaseStart(ctx)
	s.runPhase(ctx, PhaseFinalize)
	ctx.hooks.emitPhaseEnd(ctx)

	// Report errors
	if len(ctx.Errors) > 0 {
		log.Printf("Build completed with %d errors", len(ctx.Errors))
		for _, err := range ctx.Errors {
			log.Printf("  - %v", err)
		}
	}
}

// runPhase executes all rules for a specific phase.
func (s *Site) runPhase(ctx *BuildContext, phase BuildPhase) {
	rules := s.getRulesForPhase(phase)
	rules = s.topologicalSortRules(rules)

	for _, res := range ctx.Resources {
		// Skip assets - they're handled with their parent resource
		if res.AssetOf != nil {
			continue
		}

		// Skip if a rule has already claimed this resource
		if s.resourceMatchedARule(res) {
			continue
		}

		for _, rule := range rules {
			siblings, targets := rule.TargetsFor(s, res)
			if len(targets) == 0 {
				continue
			}

			s.addRuleForResource(res, rule)
			slog.Debug("Rule matched", "phase", phase, "resource", res.FullPath, "rule", rule)

			// Handle co-located assets if the rule supports it
			if assetRule, ok := rule.(AssetAwareRule); ok && len(res.Assets) > 0 {
				mappings, err := assetRule.HandleAssets(s, res, res.Assets)
				if err != nil {
					ctx.AddError(err)
				} else if err := s.processAssetMappings(mappings); err != nil {
					ctx.AddError(err)
				}
			}

			inputs := siblings
			if !slices.Contains(siblings, res) {
				inputs = append(siblings, res)
			}

			if err := rule.Run(s, inputs, targets, stageFuncs(res)); err != nil {
				log.Printf("Error running rule for %s: %v", res.FullPath, err)
				ctx.AddError(err)
			} else {
				// Track generated targets
				for _, t := range targets {
					t.ProducedBy = rule
					t.ProducedAt = phase
					ctx.AddTarget(t)
				}

				// Emit hook
				ctx.hooks.emitResourceProcessed(ctx, res, targets)
			}

			// Parametric pages are fully handled by ParametricPages rule
			if res.IsParametric {
				break
			}
		}
	}
}

// handleUnmatchedResources processes resources that didn't match any rule.
func (s *Site) handleUnmatchedResources(ctx *BuildContext) {
	for _, res := range ctx.Resources {
		// Skip assets
		if res.AssetOf != nil {
			continue
		}

		if !s.resourceMatchedARule(res) {
			rule := s.DefaultRule
			if rule != nil {
				siblings, targets := rule.TargetsFor(s, res)
				if targets == nil {
					continue
				}

				allres := append(siblings, res)
				if err := rule.Run(s, allres, targets, stageFuncs(res)); err != nil {
					log.Printf("Error in default rule for %s: %v", res.FullPath, err)
					ctx.AddError(err)
				}
			} else {
				// Copy unmatched files as-is
				respath, found := strings.CutPrefix(res.FullPath, s.ContentRoot)
				if !found {
					log.Println("Respath not found: ", res.FullPath, s.ContentRoot)
				} else {
					destpath := filepath.Join(s.OutputDir, respath)
					destres := s.GetResource(destpath)
					destres.Source = res
					destres.EnsureDir()
					data, err := res.ReadAll()
					if err != nil {
						log.Println("Could not read resource: ", res.FullPath, err)
					} else {
						os.WriteFile(destres.FullPath, data, 0666)
					}
				}
			}
		}
	}
}

func (s *Site) resourceMatchedByRule(res *Resource, rule Rule) bool {
	if s.resourceInRule[res.FullPath] == nil {
		return false
	}
	return s.resourceInRule[res.FullPath][rule]
}

func (s *Site) addRuleForResource(res *Resource, rule Rule) {
	if s.resourceInRule[res.FullPath] == nil {
		s.resourceInRule[res.FullPath] = map[Rule]bool{}
	}
	s.resourceInRule[res.FullPath][rule] = true
}

// Tells if a particular resource was "activated" by any rule.
func (s *Site) resourceMatchedARule(res *Resource) bool {
	if val, ok := s.resourceInRule[res.FullPath]; ok {
		return len(val) > 0
	}
	return false
}

func stageFuncs(res *Resource) map[string]any {
	localData := make(map[string]any)
	return map[string]any{
		"StageSet": func(key string, value any, kvpairs ...any) any {
			// log.Printf("Settin Key %s in resource %s", key, res.FullPath)
			localData[key] = value
			for i := 0; i < len(kvpairs); i += 2 {
				key = kvpairs[i].(string)
				value = kvpairs[i+1]
				localData[key] = value
				// log.Printf("Settin Key %s in resource %s", key, res.FullPath)
			}
			return ""
		},
		"StageGet": func(key string) any {
			// log.Printf("Gettin Key %s in resource %s", key, res.FullPath)
			return localData[key]
		},
		// AssetURL returns the URL for a co-located asset file.
		// For normal pages, returns relative path (./filename).
		// For parametric pages, returns shared assets path (/_assets/hash/filename).
		"AssetURL": func(filename string) string {
			return GetAssetURL(res.Site, res, filename)
		},
	}
}

func (s *Site) Serve(address string) error {
	// Attach our site to be at /`PathPrefix`
	// The site will also take care of serving static files from /`PathPrefix`/static paths
	router := mux.NewRouter()
	router.PathPrefix(s.PathPrefix).Handler(http.StripPrefix(s.PathPrefix, s))
	// router.PathPrefix(s.PathPrefix).Handler(s)

	srv := &http.Server{
		Handler: withLogger(router),
		Addr:    address,
		// Good practice: enforce timeouts for servers you create!
		// WriteTimeout: 15 * time.Second,
		// ReadTimeout:  15 * time.Second,
	}
	log.Printf("Serving site on %s:", address)
	return srv.ListenAndServe()
}

func withLogger(handler http.Handler) http.Handler {
	// the create a handler
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// pass the handler to httpsnoop to get http status and latency
		m := httpsnoop.CaptureMetrics(handler, writer, request)
		// printing exracted data
		log.Printf("http[%d]-- %s -- %s\n", m.Code, m.Duration, request.URL.Path)
	})
}

// discoverAssets identifies co-located assets for a content resource.
// Assets are determined by matching files in the same directory against
// AssetPatterns (site-level) or the "assets" frontmatter field (per-resource).
func (s *Site) discoverAssets(res *Resource) {
	// Only content files can have assets - detect by extension
	// since NeedsIndex/IsIndex aren't set until LoadResource runs later
	ext := res.Ext()
	isContentFile := ext == ".md" || ext == ".html" || ext == ".htm"
	if !isContentFile {
		return
	}

	dir := filepath.Dir(res.FullPath)

	// Get patterns: frontmatter overrides site-level
	patterns := s.AssetPatterns
	if fm := res.FrontMatter(); fm != nil && fm.Data != nil {
		if fmAssets, ok := fm.Data["assets"].([]any); ok {
			patterns = nil
			for _, p := range fmAssets {
				if ps, ok := p.(string); ok {
					patterns = append(patterns, ps)
				}
			}
		}
	}

	if len(patterns) == 0 {
		return // No asset patterns defined
	}

	// Find matching files
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			log.Printf("Error matching asset pattern %s: %v", pattern, err)
			continue
		}

		for _, match := range matches {
			if match == res.FullPath {
				continue // Skip self
			}

			asset := s.GetResource(match)
			asset.AssetOf = res
			res.Assets = append(res.Assets, asset)
		}
	}

	if len(res.Assets) > 0 {
		slog.Debug("Discovered assets", "resource", res.FullPath, "count", len(res.Assets))
	}
}

// getRulesForPhase returns all rules registered for a specific phase.
func (s *Site) getRulesForPhase(phase BuildPhase) []Rule {
	return s.PhaseRules[phase]
}

// topologicalSortRules orders rules within a phase based on their dependencies.
// Rules that produce what other rules depend on will run first.
func (s *Site) topologicalSortRules(rules []Rule) []Rule {
	if len(rules) <= 1 {
		return rules
	}

	// Build dependency graph: rule -> rules it depends on
	deps := make(map[Rule][]Rule)
	inDegree := make(map[Rule]int)

	for _, rule := range rules {
		inDegree[rule] = 0
	}

	for _, rule := range rules {
		pr, ok := rule.(PhaseRule)
		if !ok {
			continue
		}

		depends := pr.DependsOn()
		if len(depends) == 0 {
			continue
		}

		for _, other := range rules {
			if other == rule {
				continue
			}
			otherPR, ok := other.(PhaseRule)
			if !ok {
				continue
			}

			produces := otherPR.Produces()
			if patternsOverlap(depends, produces) {
				// rule depends on other
				deps[rule] = append(deps[rule], other)
				inDegree[rule]++
			}
		}
	}

	// Kahn's algorithm
	var queue []Rule
	for _, rule := range rules {
		if inDegree[rule] == 0 {
			queue = append(queue, rule)
		}
	}

	var sorted []Rule
	for len(queue) > 0 {
		rule := queue[0]
		queue = queue[1:]
		sorted = append(sorted, rule)

		// Find rules that depend on this one and decrement their in-degree
		for _, r := range rules {
			for _, dep := range deps[r] {
				if dep == rule {
					inDegree[r]--
					if inDegree[r] == 0 {
						queue = append(queue, r)
					}
					break
				}
			}
		}
	}

	// If we couldn't sort all rules, there's a cycle - return original order
	if len(sorted) != len(rules) {
		log.Println("Warning: cycle detected in rule dependencies, using original order")
		return rules
	}

	return sorted
}

// patternsOverlap checks if any pattern in set1 could match files that
// any pattern in set2 could produce.
func patternsOverlap(depends, produces []string) bool {
	for _, d := range depends {
		for _, p := range produces {
			// Simple overlap check: if patterns share common extensions or prefixes
			// A more sophisticated implementation would use proper glob matching
			dExt := filepath.Ext(d)
			pExt := filepath.Ext(p)
			if dExt != "" && pExt != "" && dExt == pExt {
				return true
			}
			// Check if one is a subset of the other
			if strings.Contains(d, p) || strings.Contains(p, d) {
				return true
			}
		}
	}
	return false
}

// processAssetMappings handles the actual copying/processing of asset files.
func (s *Site) processAssetMappings(mappings []AssetMapping) error {
	for _, m := range mappings {
		switch m.Action {
		case AssetCopy:
			destPath := filepath.Join(s.OutputDir, m.Dest)
			if err := s.copyAsset(m.Source, destPath); err != nil {
				return err
			}
		case AssetProcess:
			// TODO: Run through transform rules
			log.Printf("Asset processing not yet implemented for: %s", m.Source.FullPath)
		case AssetSkip:
			// Do nothing
		}
	}
	return nil
}

// copyAsset copies a source asset to the destination path.
func (s *Site) copyAsset(source *Resource, destPath string) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	data, err := source.ReadAll()
	if err != nil {
		// Try reading raw file for non-content assets
		data, err = os.ReadFile(source.FullPath)
		if err != nil {
			return err
		}
	}

	return os.WriteFile(destPath, data, 0644)
}
