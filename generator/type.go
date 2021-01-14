package generator

import (
	"encoding/json"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/mikekonan/go-oas3/configurator"
)

const goType = "x-go-type"

type Type struct {
	normalizer *Normalizer          `di.inject:"normalizer"`
	config     *configurator.Config `di.inject:"config"`
}

func (typ *Type) fillJsonTag(into *jen.Statement, name string) {
	into.Tag(map[string]string{"json": strings.ToLower(name[:1]) + name[1:]})
}

func (typ *Type) fillGoType(into *jen.Statement, typeName string, schemaRef *openapi3.SchemaRef, asPointer bool) {
	if asPointer {
		into.Op("*")
	}

	if schemaRef.Ref != "" {
		into.Qual(typ.config.ComponentsPackage, typ.normalizer.extractNameFromRef(schemaRef.Ref))
		return
	}

	if len(schemaRef.Value.Extensions) > 0 && schemaRef.Value.Extensions[goType] != "" {
		var customType string
		if err := json.Unmarshal(schemaRef.Value.Extensions[goType].(json.RawMessage), &customType); err != nil {
			panic(err)
		}

		into.Qual(strings.Split(customType, ".")[0], strings.Split(customType, ".")[1])
		return
	}

	schema := schemaRef.Value

	if schema.AnyOf != nil || schema.OneOf != nil || schema.AllOf != nil {
		into.Interface()
		return
	}

	if len(schema.Enum) > 0 {
		into.Qual(typ.config.ComponentsPackage, typeName)
		return
	}

	switch schema.Type {
	case "object":
		if schemaRef.Ref != "" {
			typeName := typ.normalizer.normalizeName(typ.normalizer.extractNameFromRef(schemaRef.Ref))
			into.Qual(typ.config.ComponentsPackage, typeName)
			return
		}

		if len(schema.Properties) == 0 {
			into.Interface()
			return
		}
		return
	case "array":
		into.Index()
		typ.fillGoType(into, typeName, schema.Items, asPointer)
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

func (typ *Type) isCustomType(ref *openapi3.Schema) bool {
	return ref.Type == "string" && ref.Format != ""
}
