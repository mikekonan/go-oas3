package generator

import (
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/mikekonan/go-oas3/configurator"
)

type Type struct {
	normalizer *Normalizer          `di.inject:"normalizer"`
	config     *configurator.Config `di.inject:"config"`
}

func (typ *Type) fillJsonTag(into *jen.Statement, name string) {
	into.Tag(map[string]string{"json": strings.ToLower(name[:1]) + name[1:]})
}

func (typ *Type) fillGoType(into *jen.Statement, typeName string, schemaRef *openapi3.SchemaRef) {
	if schemaRef.Ref != "" {
		into.Qual(typ.config.ComponentsPackagePath, typ.normalizer.extractNameFromRef(schemaRef.Ref))
		return
	}

	schema := schemaRef.Value

	if schema.AnyOf != nil || schema.OneOf != nil || schema.AllOf != nil {
		into.Interface()
		return
	}

	if len(schema.Enum) > 0 {
		into.Qual(typ.config.ComponentsPackagePath, typeName)
		return
	}

	switch schema.Type {
	case "object":
		if schemaRef.Ref != "" {
			typeName := typ.normalizer.normalizeName(typ.normalizer.extractNameFromRef(schemaRef.Ref))
			into.Qual(typ.config.ComponentsPackagePath, typeName)
			return
		}

		if len(schema.Properties) == 0 {
			into.Interface()
			return
		}
		return
	case "array":
		into.Index()
		typ.fillGoType(into, typeName, schema.Items)
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
		case "iso4217-currency-code":
			into.Qual("github.com/mikekonan/go-currencies", "Code")
			return
		case "iso3166-alpha-2":
			into.Qual("github.com/mikekonan/go-countries", "Alpha2Code")
			return
		case "iso3166-alpha-3":
			into.Qual("github.com/mikekonan/go-countries", "Alpha3Code")
			return
		case "uuid":
			into.Qual("github.com/google/uuid", "UUID")
			return
		case "json":
			into.Qual("encoding/json", "RawMessage")
			return
		default:
			into.String()
			return
		}
	}

	into.Interface()
}
