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

// Rebuild rebuilds the entire site. If a list of resources is provided, only
// those resources will be rebuilt.
func (s *Site) Rebuild(rs []*Resource) {
	if !s.initialized {
		s.Init()
	}
	if rs == nil {
		rs = s.ListResources(nil, nil, 0, 0)
	}

	if s.PriorityFunc != nil {
		sort.Slice(rs, func(idx1, idx2 int) bool {
			ent1 := rs[idx1]
			ent2 := rs[idx2]
			return s.PriorityFunc(ent1) < s.PriorityFunc(ent2)
		})
	}

	for _, res := range rs {
		// Skip if a rule has already claimed this resource
		if s.resourceMatchedARule(res) {
			continue
		}

		log.Println("Processing: ", res.FullPath)
		for _, rule := range s.BuildRules {
			siblings, targets := rule.TargetsFor(s, res)
			if len(targets) == 0 {
				// Rule does not apply, try the next one.
				continue
			}

			// This rule applies, so mark the resource as handled.
			s.addRuleForResource(res, rule)
			slog.Debug("Rule matched", "resource", res.FullPath, "rule", rule)

			// ** THE FIX IS HERE **
			// A single input resource can generate multiple targets (e.g., parametric pages).
			// We must iterate through each target and run the rule for it.
			inputs := siblings
			if !slices.Contains(siblings, res) {
				inputs = append(siblings, res)
			}
			// Run the rule for each individual target.
			if err := rule.Run(s, inputs, targets, stageFuncs(res)); err != nil {
				log.Println("Error running rule for resource:", res.FullPath, "targets:", len(targets), "error:", err)
			}

			// Parametric pages are fully handled by ParametricPages rule which
			// delegates internally - don't let other rules also try to match
			if res.IsParametric {
				break
			}
		}
	}

	// Now go through all resources that did NOT match any rules and pass them
	// through the default rule - which is activated when no other rules match.
	for _, res := range rs {
		if !s.resourceMatchedARule(res) {
			// log.Println("Default Matching: ", res.FullPath)
			// apply the default rule on this
			rule := s.DefaultRule
			if rule != nil {
				siblings, targets := rule.TargetsFor(s, res)
				if targets == nil {
					// rule cannot do anything with it
					continue
				}

				// Dont add the rule for this resource
				allres := append(siblings, res)
				if err := rule.Run(s, allres, targets, stageFuncs(res)); err != nil {
					log.Println("Error generating targest for res: ", res.FullPath, err)
				}
			} else {
				// log.Println("No rule found, copying: ", res.FullPath)
				// if no default rule then just copy it over
				respath, found := strings.CutPrefix(res.FullPath, s.ContentRoot)
				if !found {
					log.Println("Respath not found: ", res.FullPath, s.ContentRoot)
				} else {
					destpath := filepath.Join(s.OutputDir, respath)
					destres := s.GetResource(destpath)
					destres.Source = res
					destres.EnsureDir()
					// log.Println("Copying No resource loader for : ", res.RelPath(), ", copying over to: ", destpath)
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
