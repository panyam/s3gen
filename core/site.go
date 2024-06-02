package core

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	htmpl "html/template"
	ttmpl "text/template"

	"github.com/gorilla/mux"
	gut "github.com/panyam/goutils/utils"
	"github.com/radovskyb/watcher"
)

var ErrContentPathInvalid = fmt.Errorf("Invalid content path")

// Our static site type that holds metadata about all input directories
// output directories, implicit/explict routes, templates, etc
// Unlike a dynamic site served by go routers etc, the point of this
// is to use the same routers but to generate all this as static content
// so we can serve them directly without needing a router - faster/cdn
// and other benefits

// Usage:
//
//	router = mux.NewRouter()
//	s := Site{.....}
//	s.SetupRoutes.filesRouter)
//	srv := http.Server{
//		Processor: router,
//	 Addr: ":8080",
//	}
//	srv.ListenAndServe()
type Site struct {
	// What are the elements of a static site?

	// ContentRoot is the root of all your pages.
	// One structure we want to place is use folders to emphasis url structure too
	// so a site with mysite.com/a/b/c/d
	// would have the file structure:
	// <ContentRoot>/a/b/c/d.<content_type>
	ContentRoot string

	// Defines where all static assets are to be used from
	AssetsRoot string

	// Final output directory where resources are generated/published to
	OutputDir string

	// A directory used by the sitegen to hold cached info etc
	// TODO - easy to do just do sqlite?
	CacheDir string

	LiveReload bool
	LazyLoad   bool

	// The http path prefix the site is prefixed in,
	// eg
	//		myblog.com								=> PathPrefix = "/"
	//		myblog.com/blog						=> PathPrefix = "/blog"
	//		myblog.com/blogs/blog1		=> PathPrefix = "/blogs/blog1"
	//
	// There is no restriction on this.  There could be other routes (eg /products page)
	// that could be served by a different router all together in parallel to say /blog.
	// This is only needed so that the generator knows where to "root" the blog in the URL
	PathPrefix string

	// ResourceLoaders tell us how to "process" a content of a given type.
	// types are denoted by extensions for now later on we could do something else
	ResourceLoaders map[string]ResourceLoader

	IgnoreDirFunc  func(dirpath string) bool
	IgnoreFileFunc func(filepath string) bool

	NewViewFunc func(name string) View

	StaticFolders []string

	BuildFrequency time.Duration

	CommonFuncMap htmpl.FuncMap

	// Page callbacks are used by the site in a way that the resource being
	// rendered can provide info back to the rendered to update any state
	pageCallbacks map[string]any

	// Global templates dirs
	htmlTemplateClone *htmpl.Template
	htmlTemplate      *htmpl.Template
	HtmlFuncMap       htmpl.FuncMap
	HtmlTemplates     []string

	textTemplateClone *ttmpl.Template
	textTemplate      *ttmpl.Template
	TextFuncMap       ttmpl.FuncMap
	TextTemplates     []string

	// All files including published files will be served from here!
	filesRouter *mux.Router

	// This router wraps "published" files to ensure that when source
	// files have changed - it recompiles them before calling the underlying
	// file handler - only for non-static files
	reloadWatcher *watcher.Watcher

	resources map[string]*Resource
	pages     map[string]*Page
}

func (s *Site) Init() *Site {
	s.ContentRoot = gut.ExpandUserPath(s.ContentRoot)
	s.OutputDir = gut.ExpandUserPath(s.OutputDir)
	if s.resources == nil {
		s.resources = make(map[string]*Resource)
	}
	if s.pages == nil {
		s.pages = make(map[string]*Page)
	}
	if s.pageCallbacks == nil {
		s.pageCallbacks = make(map[string]any)
	}
	return s
}

func (s *Site) HtmlTemplateClone() *htmpl.Template {
	out, err := s.htmlTemplateClone.Clone()
	if err != nil {
		log.Println("Html Template Clone Error: ", err)
	}
	return out
}

func (s *Site) TextTemplateClone() *ttmpl.Template {
	out, err := s.textTemplateClone.Clone()
	if err != nil {
		log.Println("Text Template Clone Error: ", err)
	}
	return out
}

func (s *Site) TextTemplate() *ttmpl.Template {
	if s.textTemplate == nil {
		s.textTemplate = ttmpl.New("SiteTextTemplate").Funcs(DefaultFuncMap(s))
		if s.CommonFuncMap != nil {
			s.textTemplate = s.textTemplate.Funcs(s.CommonFuncMap)
		}
		if s.TextFuncMap != nil {
			s.textTemplate = s.textTemplate.Funcs(s.TextFuncMap)
		}
		for _, templatesDir := range s.TextTemplates {
			t, err := s.textTemplate.ParseGlob(templatesDir)
			if err != nil {
				log.Println("Error parsing templates glob: ", templatesDir)
			} else {
				s.textTemplate = t
				log.Println("Loaded Text Templates")
			}
		}
		var err error
		s.textTemplateClone, err = s.textTemplate.Clone()
		if err != nil {
			log.Println("TextTemplate Clone error: ", err)
		}
	}
	return s.textTemplate
}

func (s *Site) HtmlTemplate() *htmpl.Template {
	if s.htmlTemplate == nil {
		s.htmlTemplate = htmpl.New("SiteHtmlTemplate").Funcs(DefaultFuncMap(s))
		if s.CommonFuncMap != nil {
			s.htmlTemplate = s.htmlTemplate.Funcs(s.CommonFuncMap)
		}
		if s.HtmlFuncMap != nil {
			s.htmlTemplate = s.htmlTemplate.Funcs(s.HtmlFuncMap)
		}

		for _, templatesDir := range s.HtmlTemplates {
			slog.Info("Loaded HTML Template: ", "templatesDir", templatesDir)
			t, err := s.htmlTemplate.ParseGlob(templatesDir)
			if err != nil {
				slog.Error("Error parsing templates glob: ", "templatesDir", templatesDir, "error", err)
			} else {
				s.htmlTemplate = t
				slog.Info("Loaded HTML Template: ", "templatesDir", templatesDir)
			}
		}
		var err error
		s.htmlTemplateClone, err = s.htmlTemplate.Clone()
		if err != nil {
			log.Println("HtmlTemplate Clone error: ", err)
		}
	}
	return s.htmlTemplate
}

// https://benhoyt.com/writings/go-routing/#split-switch
func (s *Site) GetRouter() *mux.Router {
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

// Given a file in our file system - returns the "URL" that can serve that file.
func (s *Site) filePathToUrlPath(filePath string) string {
	res := s.GetResource(filePath)
	log.Println("Res, Info: ", res, res.Info())
	return ""
}

// Given a url path, eg "/a/b/c/d/e" return which "root" resource it can be
// served by.  The "root" resource is the file that will be used to compile
// and serve this file.   Eg if we have (assuming our contentRoot is ./data and
// site root is /blog
//
//	/blog/b/c/d/
//
// Then we expect the path to be at: ./data/b/c/d/<index.ext>
//
// <index.ext> could be one of the "index" files we designate,
// eg _index.html or index.html or _index.md or index.md etc
//
// For now we assume only one of these files exist and it is upto
// the page oragnizer to pick this.  An exception for this is if we want things like
func (s *Site) urlPathToFilePath( /*urlPath*/ string) *Resource {
	// cand := filepath.Join(s.ContentRoot, urlPath)
	// log.Println("Cand: ", cand)
	// if cand is directory - see if cand/<index_html> exists
	return nil
}

// The base entry point for a serving a site with our customer handler
// This is used for a few  purposes:
//  1. If you want to serve a static site but want to to have a "refresh" handler.  ie if a source changes, this can catch it and rebuild the necessary things before serving it
//     2.
func (s *Site) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The entry point router for our site
	parts := strings.Split(r.URL.Path, "/")[1:]
	log.Println("1112 - URL Parts: ", parts)
	s.GetRouter().ServeHTTP(w, r)
}

// Walks through all our resources and rebuilds/republishes the static site
func (s *Site) Load() *Site {
	foundResources := s.ListResources(nil, nil, 0, 0)
	s.Rebuild(foundResources)
	return s
}

func (s *Site) ListResources(filterFunc ResourceFilterFunc,
	sortFunc ResourceSortFunc,
	offset int, count int) []*Resource {
	var foundResources []*Resource
	// keep a map of files encountered and their statuses
	filepath.WalkDir(s.ContentRoot, func(fullpath string, info os.DirEntry, err error) error {
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
		foundResources = foundResources[:count]
	}
	return foundResources
}

// This is the heart of the build process.   This method is called with a list of resources that
// have to be reprocessed (either due to periodic updates or change events etc).   Resources in
// our site form a graph and each resource is processed by a ResourceLoader appropriate for it
// The content processor can create more resources that may need an update because they are
// dependant on this resource.   By allowing a list of resources to be processed in a batch
// it is more efficient to perform batch dependencies instead of doing repeated builds for each
// change.
// First get a transitive closure of all resource that depend on it
// then call the "build" on that resource - which in turn would load
// all of its dependencies
// So this is a closure operation on sets of resources each time
func (s *Site) Rebuild(rs []*Resource) {
	// var errors []error
	for _, res := range rs {
		/*
			if strings.HasSuffix(res.FullPath, "/index.md") {
				log.Println("Writing ", res.FullPath, "------->", outres.FullPath)
				contents, _ := res.ReadAll()
				log.Println("Contents: ", string(contents))
			}
		*/
		if false && !strings.HasSuffix(res.FullPath, "using-databases.mdx") {
			continue
		}

		// Should we just treat parametric pages separately after this loop?
		// here is where seperate pages and data may be useful?
		// pages

		var params []string
		if res.IsParametric {
			log.Println("Page Is Parametric: ", res.FullPath, res.ParamValues)
			res.LoadParamValues()
			for _, param := range res.ParamValues {
				params = append(params, param)
			}
		} else {
			params = append(params, "")
		}

		// now generate the pages here
		proc := s.GetResourceLoader(res)
		if proc == nil {
			continue
		}

		for _, param := range params {
			destpath := res.DestPathFor(param)
			outres := s.GetResource(destpath)
			if outres != nil {
				outres.EnsureDir()
				outfile, err := os.Create(outres.FullPath)
				if err != nil {
					log.Println("Error writing to: ", outres.FullPath, err)
					continue
				}
				defer outfile.Close()

				res.CurrentParamName = param
				page := res.PageFor(param)

				// Now setup the view for this page
				if page.State == ResourceStatePending {
					page.Error = proc.SetupPageView(res, page)
					if page.Error != nil {
						log.Println("Error setting up page: ", page.Error, res.FullPath)
					} else {
						page.State = ResourceStateLoaded
					}
				}

				if page.Error == nil {
					// After the page is populate, initialise it
					page.RootView.InitView(s, nil)

					// w.WriteHeader(http.StatusOK)
					err = s.RenderView(outfile, page.RootView, "")
					if err != nil {
						slog.Error("Render Error: ", "err", res.FullPath, err)
						// c.Abort()
					}
				}
			}
		}
	}
}

// Site extension to render a view
func (s *Site) RenderView(writer io.Writer, v View, templateName string) error {
	if templateName == "" {
		templateName = v.TemplateName()
	}
	if templateName != "" {
		return s.HtmlTemplate().ExecuteTemplate(writer, templateName, v)
	}
	return v.RenderResponse(writer)
}

func (s *Site) NewView(name string) (view View) {
	// TODO - register by caller or have defaults instead of hard coding
	// Leading to themes
	if s.NewViewFunc != nil {
		return s.NewViewFunc(name)
	}
	return nil
}

func (s *Site) GetResourceLoader(rs *Resource) ResourceLoader {
	// normal file
	// check type and call appropriate processor
	// Should we call processor directly here or collect a list and
	// pass that to Rebuild with those resources?
	ext := filepath.Ext(rs.FullPath)

	// TODO - move to a table lookup or regex based one
	if ext == ".mdx" || ext == ".md" {
		return NewMDResourceLoader("")
	}
	if ext == ".html" || ext == ".htm" {
		return NewHTMLResourceLoader("")
	}
	// log.Println("Could not find proc for, Name, Ext: ", rs.FullPath, ext)
	return nil
}

// Loads a resource and validates it.   Note that a resources may not
// necessarily be in memory just because it is loaded.  Just a Resource
// pointer is kept and it can be streamed etc
func (s *Site) GetResource(fullpath string) *Resource {
	res, found := s.resources[fullpath]
	if res == nil || !found {
		res = &Resource{
			Site:      s,
			FullPath:  fullpath,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			State:     ResourceStatePending,
		}
		s.resources[fullpath] = res
	}
	// Try to load it too
	res.Load()
	if res.Info() == nil {
		log.Println("Resource info is null: ", res.FullPath)
	}

	return res
}

// /////////////////// ATTIC

func (s *Site) PathToDir(path string) (string, bool, error) {
	startingDir := filepath.SplitList(s.ContentRoot)
	minPartsNeeded := len(startingDir)
	for _, p := range strings.Split(path, "/") {
		if p == "." {
			// do nothing
		} else if p == ".." {
			if len(startingDir) == minPartsNeeded {
				// cannot go any further so return error
				return "", false, ErrContentPathInvalid
			} else {
				startingDir = startingDir[:len(startingDir)-1]
			}
		} else {
			// append to it
			startingDir = append(startingDir, p)
		}
	}
	return filepath.Join(startingDir...), true, nil
}
