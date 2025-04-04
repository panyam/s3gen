package s3gen

import (
	"bytes"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	gut "github.com/panyam/goutils/utils"
	gotl "github.com/panyam/templar"
	"github.com/radovskyb/watcher"
)

// The site object is one of the most central types in s3gen.  It contains all configuration
// metadata for the site (eg input/output directories, template directories, static routes etc).
// The Site is the central point for managing the building, live reloading, templating etc needed
// to build and serve a static site.  This Site object also provides a http.HandlerFunc which can
// be used to server it via a http.Server.
type Site struct {
	Templates *gotl.TemplateGroup

	// Where our templates and loaders are
	CommonFuncMap   map[string]any
	TemplateFolders []string
	LoaderList      *gotl.LoaderList

	// ContentRoot is the root of all your pages.
	// One structure we want to place is use folders to emphasis url structure too
	// so a site with mysite.com/a/b/c/d
	// would have the file structure:
	// <ContentRoot>/a/b/c/d.<content_type>
	ContentRoot string

	// Final output directory where resources are generated/published to
	OutputDir string

	// The http path prefix the site is prefixed in,
	// The site could be served from a subpath in the domain, eg:
	// eg
	//		myblog.com								=> PathPrefix = "/"
	//		myblog.com/blog						=> PathPrefix = "/blog"
	//		myblog.com/blogs/blog1		=> PathPrefix = "/blogs/blog1"
	//
	// There is no restriction on this.  There could be other routes (eg /products page)
	// that could be served by a different router all together in parallel to say /blog.
	// This is only needed so that the generator knows where to "root" the blog in the URL
	PathPrefix string

	// A list of folders where static files could be served from along with their
	// http path prefixes.  This is an array of strings of the form
	// [ path1, folder1, path2, folder2, path3, folder3 ....]
	StaticFolders []string

	// When walking the content root for files, this callback specify which directories
	// are to be ignored.
	IgnoreDirFunc func(dirpath string) bool

	// When walking the content root for files, this callback specify which files
	// are to be ignored.
	IgnoreFileFunc func(filepath string) bool

	// Whether to enable live reload/rebuild of changed files or not
	LiveReload bool
	LazyLoad   bool

	DefaultBaseTemplate BaseTemplate
	GetTemplate         func(res *Resource, out *BaseTemplate)
	CreateResourceBase  func(res *Resource)

	BuildRules     []Rule
	DefaultRule    Rule
	resourceInRule map[string]map[Rule]bool

	BuildFrequency time.Duration

	// All files including published files will be served from here!
	filesRouter *mux.Router

	// This router wraps "published" files to ensure that when source
	// files have changed - it recompiles them before calling the underlying
	// file handler - only for non-static files
	reloadWatcher *watcher.Watcher

	resources map[string]*Resource
	resedges  map[string][]string

	initialized bool
}

// Initializes the Site
func (s *Site) Init() *Site {
	s.ContentRoot = gut.ExpandUserPath(s.ContentRoot)
	s.resourceInRule = map[string]map[Rule]bool{}
	if len(s.BuildRules) == 0 {
		// setup some defaults
		s.BuildRules = []Rule{
			&MDToHtml{BaseToHtmlRule: BaseToHtmlRule{Extensions: []string{".md", ".mdx"}}},
			&HTMLToHtml{BaseToHtmlRule: BaseToHtmlRule{Extensions: []string{".htm", ".html"}}},
		}
	}
	if s.Templates == nil {
		s.Templates = gotl.NewTemplateGroup()
		s.LoaderList = &gotl.LoaderList{}
		// Default loader is for templates
		s.LoaderList.DefaultLoader = gotl.NewFileSystemLoader(s.TemplateFolders...)
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

// Returns the full url for a path relative to the site's prefix path.
// If the Site's prefix path is /a/b/c, then PathRelUrl("d") would return /a/b/c/d
func (s *Site) PathRelUrl(path string) string {
	if s.PathPrefix == "" || s.PathPrefix == "/" {
		return path
	}
	return s.PathPrefix + path
}

// Add a new static http path and the folder from which its contents can be served.
func (s *Site) HandleStatic(path, folder string) *Site {
	s.StaticFolders = append(s.StaticFolders, path)
	s.StaticFolders = append(s.StaticFolders, folder)
	return s
}

// Returns a Router instance that can serve this as a site under a larger prefix.
func (s *Site) GetRouter() *mux.Router {
	if !s.initialized {
		s.Init()
	}
	if s.filesRouter == nil {
		s.filesRouter = mux.NewRouter()

		// Setup local/static paths
		for i := 0; i < len(s.StaticFolders); i += 2 {
			path, folder := s.StaticFolders[i], s.StaticFolders[i+1]
			log.Printf("Adding static route: %s -> %s", path, folder)
			s.filesRouter.PathPrefix(path).Handler(
				http.StripPrefix(path, http.FileServer(http.Dir(folder))))
		}

		// Serve everything else from the

		// Now add the file loader/handler for the "published" dir
		fileServer := http.FileServer(http.Dir(s.OutputDir))
		realHandler := http.StripPrefix("/", fileServer)
		if s.LazyLoad {
			x := s.filesRouter.PathPrefix("/")
			x.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.Println("Ensuring path is built: ", r.URL.Path, "Site Prefix: ", s.PathPrefix)

				// srcRes := s.urlPathToFilePath(r.URL.Path)
				// log.Println("Source Resource: ", srcRes)
				// What should happen here?
				realHandler.ServeHTTP(w, r)
			}))
		} else {
			x := s.filesRouter.PathPrefix("/")
			x.Handler(realHandler)
		}
	}
	return s.filesRouter
}

// The base entry point for a serving a site with our customer handler -
// also implementing the http.Handler interface
func (s *Site) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The entry point router for our site
	// parts := strings.Split(r.URL.Path, "/")[1:]
	// log.Println("1112 - URL Parts: ", parts)
	s.GetRouter().ServeHTTP(w, r)
}

// A method to list all the resource in our site in the content root.  This method also allows pagination, filtering and sorting of resources.
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

		if filterFunc == nil && s.IgnoreFileFunc != nil && s.IgnoreFileFunc(fullpath) {
			return nil
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

func (s *Site) GenerateSitemap() map[string]any {
	return nil
}

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

func (s *Site) Rebuild(rs []*Resource) {
	if !s.initialized {
		s.Init()
	}
	if rs == nil {
		rs = s.ListResources(nil, nil, 0, 0)
	}

	for _, res := range rs {
		if false && res.FullPath != "/Users/sri/personal/golang/leetcoach/content/casestudies/bitly/index.md" {
			continue
		}
		for _, rule := range s.BuildRules {
			if s.resourceMatchedByRule(res, rule) {
				// this rule has already matched this resource and will be processed
				// so skip this rule
				continue
			}

			// now see if this rule can process this rule
			siblings, targets := rule.TargetsFor(s, res)
			if targets == nil {
				// rule cannot do anything with it
				continue
			}

			// TODO - See if cycle checking needed

			s.addRuleForResource(res, rule)

			// And run the rule
			allres := append(siblings, res)
			if err := rule.Run(s, allres, targets, stageFuncs(res)); err != nil {
				log.Println("Error generating targest for res: ", res.FullPath, err)
			}
		}
	}

	// Now go through all resources that did NOT match any rules and pass them through the default rule - which is
	// activated when no other rules match
	for _, res := range rs {
		if res.Ext() == ".md" || res.Ext() == ".html" {
			continue
		}
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

// Tells if a particular sibling was "Activated" by a given rule
func (s *Site) resourceMatchedARule(res *Resource) bool {
	if s.resourceInRule[res.FullPath] == nil {
		return false
	}
	return true
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
