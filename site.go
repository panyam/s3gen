package s3gen

import (
	"bytes"
	"fmt"
	htmpl "html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gorilla/mux"
	gfn "github.com/panyam/goutils/fn"
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

	// ResourceHandlers tell us how to "process" a content of a given type.
	// types are denoted by extensions for now later on we could do something else
	ResourceHandlers map[string]ResourceHandler

	// When walking the content root for files, this callback specify which directories
	// are to be ignored.
	IgnoreDirFunc func(dirpath string) bool

	// When walking the content root for files, this callback specify which files
	// are to be ignored.
	IgnoreFileFunc func(filepath string) bool

	// Whether to enable live reload/rebuild of changed files or not
	LiveReload bool
	LazyLoad   bool

	DefaultPageTemplate PageTemplate
	GetTemplate         func(res *Resource, out *PageTemplate)
	CreatePage          func(res *Resource)

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
	s.OutputDir = gut.ExpandUserPath(s.OutputDir)
	if s.CreatePage == nil {
		s.CreatePage = func(res *Resource) {
			p := &DefaultPage{Res: res, Site: res.Site}
			res.Page = p
			if err := p.LoadFrom(res); err != nil {
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

// This is the heart of the build process.   This method is called with a list of resources that
// have to be reprocessed (either due to periodic updates or change events etc).   Resources in
// our site form a graph and each resource is processed by a ResourceHandler appropriate for it
// The content processor can create more resources that may need an update because they are
// dependant on this resource.   By allowing a list of resources to be processed in a batch
// it is more efficient to perform batch dependencies instead of doing repeated builds for each
// change.
func (s *Site) Rebuild(rs []*Resource) {
	if !s.initialized {
		s.Init()
	}
	if rs == nil {
		rs = s.ListResources(nil, nil, 0, 0)
	}

	dest_dep_res := make(map[string]*Resource)
	// Step 1 - Update dependencies and collect affected outputs
	for _, res := range rs {
		// now generate the pages here
		proc := s.GetResourceHandler(res)
		if proc == nil {
			log.Println("No resource loader for : ", res.RelPath())
			// TODO - may be do a defaault handler?
			continue
		}

		err := proc.LoadResource(res)
		if err != nil {
			log.Println("Error loading resource: ", res.FullPath, err)
			continue
		}

		// The site maintains a dependency graph between resources.
		// The handler is responsible for updating this dependency list for a given
		// resource so we can track changes, rebuilds etc

		// For every resource - there can be one or more "destination" resources
		// eg a/b/c/d.md its dest resource would be outdir/a/b/c/d/index.html
		// if it is parametric it can have several destination resources
		// eg a/b/[animal].md could have a/b/cat/index.html, a/b/dog/index.html and so on
		// So we need to see if the resource is "final" in which case render it, otherwise
		// return child resources - that depends on the parent
		if res.IsParametric {
			slog.Info("Resource Is Parametric: ", "filepath", res.FullPath, "paramvalues", res.ParamValues)
			err = proc.LoadParamValues(res)
			if err != nil {
				log.Println("Error loading param values: ", res.FullPath, err)
				continue
			}
		}

		// Now generate the dependent resources and add to our list
		_ = proc.GenerateTargets(res, dest_dep_res)
	}

	// Step 2: Re-Render all affected outputs
	for _, outres := range dest_dep_res {
		inres := outres.Source
		proc := s.GetResourceHandler(inres)

		outres.EnsureDir()
		outfile, err := os.Create(outres.FullPath)
		if err != nil {
			log.Println("Error writing to: ", outres.FullPath, err)
			continue
		}
		defer outfile.Close()

		// Now setup the view for this parameter specific resource
		contentBuffer := bytes.NewBufferString("")

		// Bit of a hack - though we rendering the "source", we
		// need the paramname to be set - and it is only set in outres
		// so we are temporarily setting it in inres too
		// TODO - Need a way around this hack - ie somehow call RenderContent
		// on outres with the right expectation
		inres.ParamName = outres.ParamName
		err = proc.RenderContent(inres, contentBuffer)
		inres.ParamName = ""
		if err != nil {
			slog.Warn("Content rendering failed: ", "err", err, "path", outres.FullPath)
		} else {
			_ = proc.RenderResource(outres, contentBuffer.String(), outfile)
		}
	}
}

// Every resource needs a ResourceHandler.  This is a factory method to return one given a resource.
// TODO - Today there is no way to override this.  In the future this will be turned into a method attribute
func (s *Site) GetResourceHandler(rs *Resource) ResourceHandler {
	// normal file
	// check type and call appropriate processor
	// Should we call processor directly here or collect a list and
	// pass that to Rebuild with those resources?
	ext := filepath.Ext(rs.FullPath)

	// TODO - move to a table lookup or regex based one
	if ext == ".mdx" || ext == ".md" {
		return NewMDResourceHandler("")
	}
	if ext == ".html" || ext == ".htm" {
		return NewHTMLResourceHandler("")
	}
	// log.Println("Could not find proc for, Name, Ext: ", rs.FullPath, ext)
	return nil
}

// Starts watching for changes to content files so that the site can be rebuilt.
func (s *Site) Watch() {
	// Always build once
	s.Rebuild(nil)

	if s.reloadWatcher == nil {
		w := watcher.New()
		s.reloadWatcher = w

		go func() {
			buildFreq := s.BuildFrequency
			if buildFreq <= 0 {
				buildFreq = 1000 * time.Millisecond
			}
			tickerChan := time.NewTicker(buildFreq)
			defer tickerChan.Stop()

			foundResources := make(map[string]*Resource)
			for {
				select {
				case event := <-w.Event:
					fmt.Println(event) // Print the event's info.
					log.Println("Collecting Event: ", event)

					fullpath := event.Path
					info, err := os.Stat(fullpath)
					if err != nil {
						fmt.Println("Error with file: ", event.Path, err)
						continue
					}

					// only deal with files
					if !info.IsDir() && (s.IgnoreFileFunc == nil || !s.IgnoreFileFunc(fullpath)) {
						res := s.GetResource(fullpath)
						if res != nil {
							// map fullpath to a resource here

							// TODO - refer to cache if this need to be rebuilt? or let Rebuild do it?
							foundResources[fullpath] = res
							res.Reset()
						}
					}
				case err := <-w.Error:
					log.Fatalln(err)
				case <-w.Closed:
					// Stop building and uit
					return
				case <-tickerChan.C:
					// if we have things in the collected files - kick off a rebuild
					if len(foundResources) > 0 {
						log.Println("files collected so far: ", foundResources)

						s.Rebuild(gfn.MapValues(foundResources))
						// reset changed files
						foundResources = make(map[string]*Resource)
					}
					break
				}
			}
		}()

		log.Println("Adding files recursive: ", s.ContentRoot)
		if err := w.AddRecursive(s.ContentRoot); err != nil {
			log.Fatalln("Error adding files recursive: ", s.ContentRoot, err)
		}

		// start the watching process
		go func() {
			log.Println("Starting watcher...")
			if err := w.Start(time.Millisecond * 100); err != nil {
				log.Fatal("Error starting watcher...", err)
			}
		}()
	}
}

// Disables/Stops watching for changes to content files.
func (s *Site) StopWatching() {
	if s.reloadWatcher == nil {
		s.reloadWatcher.Close()
		s.reloadWatcher = nil
	}
}

func (s *Site) DefaultFuncMap() htmpl.FuncMap {
	out := s.Templates.DefaultFuncMap()
	out["json"] = s.Json
	return out
}

func (s *Site) Json(path string, fieldpath string) (any, error) {
	if path[0] == '/' {
		return nil, fmt.Errorf("Invalid json file: %s.  Cannot start with a /", path)
	}
	fullpath := gut.ExpandUserPath(filepath.Join(s.ContentRoot, path))
	res := s.GetResource(fullpath)
	if res.Ext() != ".json" {
		return nil, fmt.Errorf("Invalid json file: %s, Ext: %s", fullpath, res.Ext())
	}

	data, err := res.ReadAll()
	if err != nil {
		return nil, err
	}
	out, err := gut.JsonDecodeBytes(data)
	if err != nil {
		log.Println("Error Decoding Json: ", path, err)
	}
	return out, err
}
