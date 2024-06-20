package s3gen

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"time"

	gut "github.com/panyam/goutils/utils"
)

func DefaultFuncMap(s *Site) template.FuncMap {
	return template.FuncMap{
		"Now":         time.Now,
		"HTML":        func(s string) template.HTML { return template.HTML(s) },
		"JS":          func(s string) template.JS { return template.JS(s) },
		"URL":         func(s string) template.URL { return template.URL(s) },
		"RangeN":      func(n int) []struct{} { return make([]struct{}, n) },
		"TypeOf":      reflect.TypeOf,
		"ExpandAttrs": ExpandAttrs,
		"Slice":       SliceArray,
		"AddInts":     AddNums[int],
		"MultInts":    MultNums[int],
		"SubInt":      SubNums[int],
		"DivInt":      IntDiv[int, int],
		"String":      ToString,
		"Int":         ToInt,
		"Float":       ToFloat,
		"IntList":     NumList[int],
		"Add": func(vals ...any) (out float64) {
			for _, v := range vals {
				out += ToFloat(v)
			}
			return
		},
		"Multiply": func(vals ...any) (out float64) {
			for _, v := range vals {
				out *= ToFloat(v)
			}
			return
		},
		"Sub": func(a any, b any) (out float64) { return ToFloat(a) - ToFloat(b) },
		"Div": func(a any, b any) float64 { return ToFloat(a) / ToFloat(b) },
		"Floor": func(val float64) int64 {
			return int64(val)
		},
		"Ceil": func(val float64) int {
			return int(val + 0.5)
		},

		"RenderHtml": func(templateName string, params map[string]any) (template.HTML, error) {
			out := bytes.NewBufferString("")
			err := s.HtmlTemplate().ExecuteTemplate(out, templateName, params)
			return template.HTML(out.String()), err
		},

		"RenderView": func(view View) (out template.HTML, err error) {
			if view == nil {
				return "", fmt.Errorf("view is nil")
			} else {
				// log.Println("Rendering View: ", view, reflect.TypeOf(view))
			}
			defer func() {
				if false {
					return
				}
				if r := recover(); r != nil {
					log.Println("========================================================")
					debug.PrintStack()
					if e, ok := r.(error); ok {
						err = e
					} else {
						err = fmt.Errorf("%v", r)
					}
				}
			}()
			output := bytes.NewBufferString("")
			err = view.RenderResponse(output)
			return template.HTML(output.String()), err
		},

		"RenderText": func(templateName string, params map[string]any) (string, error) {
			out := bytes.NewBufferString("")
			err := s.TextTemplate().ExecuteTemplate(out, templateName, params)
			return out.String(), err
		},

		"JoinA": func(delim string, parts []string) string {
			return strings.Join(parts, delim)
		},
		"Join": func(delim string, parts ...string) string {
			return strings.Join(parts, delim)
		},
		"Split":     strings.Split,
		"HasPrefix": strings.HasPrefix,
		"HasSuffix": strings.HasSuffix,
		"Slugify":   Slugify,
		"dict":      ValuesToDict,
		"json": func(path string, fieldpath string) (any, error) {
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
			return gut.JsonDecodeBytes(data)
		},
	}
}
