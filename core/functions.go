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

func AddNums[T float32 | int | int32 | float64 | int64 | uint64 | uint32 | uint](values ...T) (out T) {
	for _, v := range values {
		out += v
	}
	return
}

func MultNums[T float32 | int | int32 | float64 | int64 | uint64 | uint32 | uint](values ...T) (out T) {
	for _, v := range values {
		out *= v
	}
	return
}

func SubNums[T float32 | int | int32 | float64 | int64 | uint64 | uint32 | uint](a, b T) (out T) {
	return a - b
}

type Number interface {
	float32 | int | int32 | float64 | int64 | uint64 | uint32 | uint
}

func FloatDiv[A Number, B Number](a A, b B) (out float64) {
	return float64(a) / float64(b)
}

func IntDiv[A Number, B Number](a A, b B) (out int64) {
	return int64(FloatDiv(a, b))
}

func ToInt(v any) int64 {
	if val, ok := v.(int); ok {
		return int64(val)
	}
	if val, ok := v.(int8); ok {
		return int64(val)
	}
	if val, ok := v.(int16); ok {
		return int64(val)
	}
	if val, ok := v.(int32); ok {
		return int64(val)
	}
	if val, ok := v.(int64); ok {
		return val
	}
	if val, ok := v.(uint8); ok {
		return int64(val)
	}
	if val, ok := v.(uint16); ok {
		return int64(val)
	}
	if val, ok := v.(uint); ok {
		return int64(val)
	}
	if val, ok := v.(uint32); ok {
		return int64(val)
	}
	if val, ok := v.(uint64); ok {
		return int64(val)
	}
	if val, ok := v.(float32); ok {
		return int64(val)
	}
	if val, ok := v.(float64); ok {
		return int64(val)
	}
	// Todo check string too
	return 0
}

func ToFloat(v any) float64 {
	if val, ok := v.(int); ok {
		return float64(val)
	}
	if val, ok := v.(int8); ok {
		return float64(val)
	}
	if val, ok := v.(int16); ok {
		return float64(val)
	}
	if val, ok := v.(int32); ok {
		return float64(val)
	}
	if val, ok := v.(int64); ok {
		return float64(val)
	}
	if val, ok := v.(uint8); ok {
		return float64(val)
	}
	if val, ok := v.(uint16); ok {
		return float64(val)
	}
	if val, ok := v.(uint); ok {
		return float64(val)
	}
	if val, ok := v.(uint32); ok {
		return float64(val)
	}
	if val, ok := v.(uint64); ok {
		return float64(val)
	}
	if val, ok := v.(float32); ok {
		return float64(val)
	}
	if val, ok := v.(float64); ok {
		return val
	}

	// Todo - do string as well
	return 0
}

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

		"Slice": func(values any, offset, count int) any {
			v := reflect.ValueOf(values)
			t := reflect.TypeOf(values)
			// log.Println("T: ", t, t.Kind(), reflect.Slice, reflect.Array)
			if t.Kind() == reflect.Array || t.Kind() == reflect.Slice || t.Kind() == reflect.String {
				return v.Slice(offset, offset+count).Interface()
			}
			return nil
		},
		"AddInts":  AddNums[int],
		"MultInts": MultNums[int],
		"SubInt":   SubNums[int],
		"DivInt":   IntDiv[int, int],
		"Float":    ToFloat,
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
		"Ceil": func(val float64) int64 {
			return int64(val + 0.5)
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
