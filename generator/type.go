package generator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/mikekonan/go-oas3/configurator"
)

const (
	goType            = "x-go-type"
	goTypeStringParse = "x-go-type-string-parse"
	goRegex           = "x-go-regex"
)

type Type struct {
	normalizer *Normalizer          `di.inject:"normalizer"`
	config     *configurator.Config `di.inject:"config"`
}

func (typ *Type) fillJsonTag(into *jen.Statement, name string) {
	into.Tag(map[string]string{"json": strings.ToLower(name[:1]) + name[1:]})
}

func (typ *Type) fillGoType(into *jen.Statement, typeName string, schemaRef *openapi3.SchemaRef, asPointer bool, needAliasing bool) {
	if typeName == "CurrencyCode" {
		fmt.Print()
	}
	if asPointer {
		into.Op("*")
	}

	if pkg, typee, ok := typ.getXGoType(schemaRef.Value); ok {
		if needAliasing {
			into.Op("=")
		}

		into.Qual(pkg, typee)
		return
	}

	if schemaRef.Ref != "" {
		into.Qual(typ.config.ComponentsPackage, typ.normalizer.extractNameFromRef(schemaRef.Ref))
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
			typeName := typ.normalizer.normalize(typ.normalizer.extractNameFromRef(schemaRef.Ref))
			into.Qual(typ.config.ComponentsPackage, typeName)
			return
		}

		if schema.AdditionalProperties != nil {
			into.Map(jen.Id("string"))
			typ.fillGoType(into, typeName, schema.AdditionalProperties, false, needAliasing)
			return
		}

		if len(schema.Properties) == 0 {
			into.Interface()
			return
		}
		return
	case "array":
		into.Index()
		typ.fillGoType(into, typeName, schema.Items, false, needAliasing)
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
		if needAliasing {
			into.Op("=")
		}

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

func (typ *Type) hasXGoType(schema *openapi3.Schema) bool {
	if len(schema.Extensions) > 0 && schema.Extensions[goType] != nil {
		return true
	}

	return false
}

func (typ *Type) hasXGoTypeStringParse(schema *openapi3.Schema) bool {
	if typ.hasXGoType(schema) && schema.Extensions[goTypeStringParse] != nil {
		return true
	}

	return false
}

func (typ *Type) getXGoTypeStringParse(schema *openapi3.Schema) (string, string, bool) {
	if typ.hasXGoType(schema) && schema.Extensions[goTypeStringParse] != nil {
		var customType string

		if err := json.Unmarshal(schema.Extensions[goTypeStringParse].(json.RawMessage), &customType); err != nil {
			panic(err)
		}

		index := strings.LastIndex(customType, ".")

		return customType[:index], customType[index+1:], true
	}

	return "", "", false
}

func (typ *Type) getXGoType(schema *openapi3.Schema) (string, string, bool) {
	if typ.hasXGoType(schema) && schema.Extensions[goType] != nil {
		var customType string

		if err := json.Unmarshal(schema.Extensions[goType].(json.RawMessage), &customType); err != nil {
			panic(err)
		}

		index := strings.LastIndex(customType, ".")

		return customType[:index], customType[index+1:], true
	}

	return "", "", false
}

func (typ *Type) isCustomType(schema *openapi3.Schema) bool {
	return schema.Type == "string" && (schema.Format != "" || typ.hasXGoTypeStringParse(schema))
}
