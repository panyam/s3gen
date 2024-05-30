package core

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"log"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	gut "github.com/panyam/goutils/utils"
)

func DefaultFuncMap(s *Site) template.FuncMap {
	return template.FuncMap{
		"Now": time.Now,

		"HTML": func(s string) template.HTML {
			return template.HTML(s)
		},

		"JS": func(s string) template.JS {
			return template.JS(s)
		},

		"URL": func(s string) template.URL {
			return template.URL(s)
		},

		"TypeOf": reflect.TypeOf,

		"expandAttrs": func(attrs map[string]any) template.JS {
			out := " "
			if attrs != nil {
				for key, value := range attrs {
					val := fmt.Sprintf("%v", value)
					val = strings.Replace(val, "\"", "&quot;", -1)
					val = strings.Replace(val, "\"", "&quot;", -1)
					out += " " + fmt.Sprintf("%s = \"%s\"", key, val)
				}
			}
			return template.JS(out)
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
		"Slugify": func(input string) string {
			// Remove special characters
			reg, err := regexp.Compile("[^a-zA-Z0-9]+")
			if err != nil {
				panic(err)
			}
			processedString := reg.ReplaceAllString(input, " ")

			// Remove leading and trailing spaces
			processedString = strings.TrimSpace(processedString)

			// Replace spaces with dashes
			slug := strings.ReplaceAll(processedString, " ", "-")

			// Convert to lowercase
			slug = strings.ToLower(slug)

			return slug
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},

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
