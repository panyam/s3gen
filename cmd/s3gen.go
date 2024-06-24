package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/mux"
	"github.com/panyam/s3gen"
)

var (
	config_file        = flag.String("config_file", "config.yaml", "Config file to load site config from")
	content_root       = flag.String("content_root", "", "Root folder from which all content is built from.  Will override the respective value in the config if provided")
	output_dir         = flag.String("output_dir", "", "If specified the output_dir in the config will be overridden with this")
	site_prefix        = flag.String("site_prefix", "", "By default sites can be served at /.  This determines the site prefix to use and will override the respective value in the config if provided")
	html_templates_dir = flag.String("html_templates_dir", "", "List of Folders from html templates will be parsed and used.")
	text_templates_dir = flag.String("text_templates_dir", "", "List of Folders from text templates will be parsed and used.")
	serve_addr         = flag.String("serve_addr", "", "The address on which to serve this site (with live reloading enabled) in dev mode")
)

func main() {
	fmt.Println("A simple static site generator")
	site := s3gen.Site{}
	if *serve_addr != "" {
		site.Watch()
		// Attach our site to be at /`PathPrefix`
		// The site will also take care of serving static files from /`PathPrefix`/static paths
		router := mux.NewRouter()
		router.PathPrefix(site.PathPrefix).Handler(http.StripPrefix(site.PathPrefix, &site))

		srv := &http.Server{
			Handler: withLogger(router),
			Addr:    *serve_addr,
			// Good practice: enforce timeouts for servers you create!
			// WriteTimeout: 15 * time.Second,
			// ReadTimeout:  15 * time.Second,
		}
		log.Printf("Serving Site on %s:", *serve_addr)
		log.Fatal(srv.ListenAndServe())
	}
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
