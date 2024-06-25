package s3gen

import (
	"github.com/morrisxyang/xreflect"
)

func SetNestedProp(obj any, value any, fieldpath string) error {
	return xreflect.SetEmbedField(obj, fieldpath, value)
}
