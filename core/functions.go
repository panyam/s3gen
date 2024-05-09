package core

import (
	"errors"
	"fmt"
	"html/template"
	"strings"
	"time"
)

func DefaultFuncMap() template.FuncMap {
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
	}
}
