package transformer

import (
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
)

type TypeFiller struct {
	normalizer *Normalizer `di.inject:"normalizer"`
}

func (filler *TypeFiller) fillJsonTag(into *jen.Statement, name string) {
	into.Tag(map[string]string{"json": strings.ToLower(name[:1]) + name[1:]})
}

func (filler *TypeFiller) fillGoType(into *jen.Statement, schemaRef *openapi3.SchemaRef) {
	schema := schemaRef.Value

	if schema.AnyOf != nil || schema.OneOf != nil || schema.AllOf != nil {
		into.Interface()
		return
	}

	switch schema.Type {
	case "object":
		if schemaRef.Ref != "" {
			typeName := filler.normalizer.normalizeName(filler.normalizer.extractNameFromRef(schemaRef.Ref))
			into.Id(typeName)
			return
		}

		if len(schema.Properties) == 0 {
			into.Interface()
			return
		}
		return
	case "array":
		into.Index()
		filler.fillGoType(into, schema.Items)
		return
	case "integer":
		into.Int()
		return
	case "number":
		into.Float64()
		return
	case "boolean":
		into.Bool()
		return
	case "string":
		switch schema.Format {
		case "byte":
			into.Byte().Values()
			return
		case "email":
			into.String()
			return
		case "date":
			into.String()
			return
		case "date-time":
			into.String()
			return
		case "uuid":
			into.Id("uuid").Dot("UUID")
			return
		case "json":
			into.Id("json").Dot("RawMessage")
			return
		default:
			into.String()
			return
		}
	}

	into.Interface()
}
