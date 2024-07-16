package s3gen

import (
	"github.com/morrisxyang/xreflect"
)

func setNestedProp(obj any, value any, fieldpath string) error {
	return xreflect.SetEmbedField(obj, fieldpath, value)
}
