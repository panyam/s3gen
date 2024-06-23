package funcs

import (
	"errors"
	"fmt"
	"html/template"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

func NumList[T float32 | int | int32 | float64 | int64 | uint64 | uint32 | uint](start, end, incr int) (out []T) {
	if incr > 0 {
		for i := start; i <= end; i += incr {
			out = append(out, T(i))
		}
	} else if incr < 0 {
		for i := start; i >= end; i += incr {
			out = append(out, T(i))
		}
	}
	return
}

func AddNums[T float32 | int | int32 | float64 | int64 | uint64 | uint32 | uint](values ...T) (out T) {
	for _, v := range values {
		out += v
	}
	return
}

func MultNums[T float32 | int | int32 | float64 | int64 | uint64 | uint32 | uint](values ...T) (out T) {
	out = 1
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

func ToString(v any) string {
	if val, ok := v.(string); ok {
		return val
	}
	return fmt.Sprintf("%v", v)
}

func ToInt(v any) int {
	if val, ok := v.(int); ok {
		return int(val)
	}
	if val, ok := v.(int8); ok {
		return int(val)
	}
	if val, ok := v.(int16); ok {
		return int(val)
	}
	if val, ok := v.(int32); ok {
		return int(val)
	}
	if val, ok := v.(int64); ok {
		return int(val)
	}
	if val, ok := v.(uint8); ok {
		return int(val)
	}
	if val, ok := v.(uint16); ok {
		return int(val)
	}
	if val, ok := v.(uint); ok {
		return int(val)
	}
	if val, ok := v.(uint32); ok {
		return int(val)
	}
	if val, ok := v.(uint64); ok {
		return int(val)
	}
	if val, ok := v.(float32); ok {
		return int(val)
	}
	if val, ok := v.(float64); ok {
		return int(val)
	}
	if val, ok := v.(string); ok {
		out, _ := strconv.Atoi(val)
		return out
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

func SliceArray(values any, offset, count any) any {
	v := reflect.ValueOf(values)
	t := reflect.TypeOf(values)
	// log.Println("T: ", t, t.Kind(), reflect.Slice, reflect.Array)
	if t.Kind() == reflect.Array || t.Kind() == reflect.Slice || t.Kind() == reflect.String {
		// log.Println("Before Off, Cnt, Len: ", offset, count, v.Len())
		start := ToInt(offset)
		if start < 0 {
			start = 0
		}
		end := ToInt(count)
		l := v.Len()
		if end < 0 {
			end = l + end + 1
		} else if start+end > v.Len() {
			end = l
		} else {
			end = start + end
		}

		out := v.Slice(start, end)
		// log.Println("After Slice Off, Cnt, Len: ", start, end, v.Len(), out.Len(), out)
		return out.Interface()
	}
	return nil
}

func ExpandAttrs(attrs map[string]any) template.JS {
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
}

func Slugify(input string) string {
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
}

func ValuesToDict(values ...interface{}) (map[string]interface{}, error) {
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
}
