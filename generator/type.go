package generator

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"dario.cat/mergo"
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mikekonan/go-oas3/configurator"
)

const (
	goType              = "x-go-type"
	goMapType           = "x-go-map-type"
	goTypeStringParse   = "x-go-type-string-parse"
	goPointer           = "x-go-pointer"
	goRegex             = "x-go-regex"
	goStringTrimmable   = "x-go-string-trimmable"
	goOmitempty         = "x-go-omitempty"
	goSkipValidation    = "x-go-skip-validation"
	goSkipSecurityCheck = "x-go-skip-security-check"
)

type Type struct {
	normalizer *Normalizer          `di.inject:"normalizer"`
	config     *configurator.Config `di.inject:"config"`
}

func (typ *Type) fillJsonTag(into *jen.Statement, schemaRef *openapi3.SchemaRef, name string) {
	tag := formatTagName(name)
	if schemaRef.Value != nil && typ.getXGoOmitempty(schemaRef.Value) {
		tag += ",omitempty"
	}
	into.Tag(map[string]string{"json": tag})
}

func (typ *Type) fillAdditionalProperties(into *jen.Statement, schema *openapi3.Schema) {
	// Sort property names to ensure deterministic ordering
	var propNames []string
	for propName := range schema.Properties {
		propNames = append(propNames, propName)
	}
	slices.Sort(propNames)

	for _, propName := range propNames {
		propSchema := schema.Properties[propName]
		fieldName := typ.normalizer.normalize(propName)
		field := jen.Id(fieldName)

		typ.fillGoType(field, "", fieldName, propSchema, false, false)
		typ.fillJsonTag(field, propSchema, propName)
		into.Line().Add(field)
	}
}

func (typ *Type) fillGoType(into *jen.Statement, parentTypeName string, typeName string, schemaRef *openapi3.SchemaRef, asPointer bool, needAliasing bool) {
	if asPointer || typ.getXGoPointer(schemaRef.Value) {
		into.Op("*")
	}

	if pkg, typee, ok := typ.getXGoType(schemaRef.Value); ok && schemaRef.Value.AdditionalProperties == nil {
		if needAliasing {
			into.Op("=")
		}

		if pkg == "" {
			into.Id(typee)
			return
		}
		into.Qual(pkg, typee)
		return
	}

	schema := schemaRef.Value

	if schema.AnyOf != nil || schema.OneOf != nil {
		into.Interface()
		return
	}

	if schemaRef.Ref != "" {
		into.Qual(typ.config.ComponentsPackage, typ.normalizer.extractNameFromRef(schemaRef.Ref))
		return
	}

	if len(schema.AllOf) > 0 {
		var refSchema *openapi3.SchemaRef
		var inlineSchemas []*openapi3.SchemaRef

		for _, s := range schema.AllOf {
			if s.Ref != "" {
				if refSchema == nil {
					refSchema = s
				}
			} else {
				inlineSchemas = append(inlineSchemas, s)
			}
		}

		if refSchema != nil {
			typ.fillGoType(into, parentTypeName, typeName, refSchema, false, needAliasing)

			if len(inlineSchemas) > 0 {
				mergedInline := &openapi3.Schema{}
				for _, s := range inlineSchemas {
					if err := mergo.Merge(mergedInline, s.Value, mergo.WithOverride); err != nil {
						panic(err)
					}
				}
				typ.fillAdditionalProperties(into, mergedInline)
			}

			return
		}

		mergedSchema := &openapi3.Schema{}
		for _, s := range schema.AllOf {
			if s.Value == nil {
				continue
			}

			if err := mergo.Merge(mergedSchema, s.Value, mergo.WithOverride); err != nil {
				panic(err)
			}
		}
		typ.fillGoType(into, parentTypeName, typeName, &openapi3.SchemaRef{Value: mergedSchema}, false, needAliasing)
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
			keyCode := jen.Null()

			keyPkg, keyValue, ok := typ.getXGoType(schemaRef.Value)
			if ok {
				if keyPkg == "" {
					keyCode.Id(keyValue)
				} else {
					keyCode.Qual(keyPkg, keyValue)
				}
			} else {
				keyCode.String()
			}

			into.Map(keyCode)

			typ.fillGoType(into, parentTypeName, typeName, schema.AdditionalProperties, false, false)

			//TODO: ANONYMOUS MAP ENTRIES
			//if schema.AdditionalProperties.Ref != "" {
			//	typ.fillGoType(into, parentTypeName, typeName, schema.AdditionalProperties, false, needAliasing)
			//	return
			//}

			//into.Qual(typ.config.ComponentsPackage, parentTypeName+typeName+"MapEntry")

			return
		}

		if len(schema.Properties) == 0 {
			into.Interface()
			return
		}
		return
	case "array":
		into.Index()

		//TODO: ANONYMOUS SLICES
		//if schema.Items.Ref != "" {
		//	typ.fillGoType(into, parentTypeName, typeName, schema.Items, false, needAliasing)
		//	return
		//}

		//into.Qual(typ.config.ComponentsPackage, parentTypeName+typeName+"SliceElement")

		typ.fillGoType(into, parentTypeName, typeName, schema.Items, false, needAliasing)
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
			into.Index().Byte()
			return
		case "binary":
			into.Index().Byte()
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
			into.Qual("github.com/mikekonan/go-types/v2/currency", "Code")
			return
		case "iso3166-alpha-2":
			into.Qual("github.com/mikekonan/go-types/v2/country", "Alpha2Code")
			return
		case "iso3166-alpha-3":
			into.Qual("github.com/mikekonan/go-types/v2/country", "Alpha3Code")
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

func (typ *Type) hasXGoMapType(schema *openapi3.Schema) bool {
	if len(schema.Extensions) > 0 && schema.Extensions[goMapType] != nil {
		return true
	}

	return false
}

func (typ *Type) hasXGoPointer(schema *openapi3.Schema) bool {
	if len(schema.Extensions) > 0 && schema.Extensions[goPointer] != nil {
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
	if typ.hasXGoTypeStringParse(schema) {
		var customType string

		if err := json.Unmarshal(schema.Extensions[goTypeStringParse].(json.RawMessage), &customType); err != nil {
			panic(err)
		}

		index := strings.LastIndex(customType, ".")

		return customType[:index], customType[index+1:], true
	}

	return "", "", false
}

func (typ *Type) getXGoMapType(schema *openapi3.Schema) (keyPkg string, key string, keyIsType bool, valuePkg string, value string, valueIsType bool, isValueArr bool, found bool) {
	if typ.hasXGoMapType(schema) {
		var customType string

		if err := json.Unmarshal(schema.Extensions[goMapType].(json.RawMessage), &customType); err != nil {
			panic(err)
		}

		if strings.HasPrefix(customType, "map[") {
			keyPkg = customType[4:]
			keyPkg = keyPkg[:strings.Index(keyPkg, "]")]
			dotIndex := strings.LastIndex(keyPkg, ".")
			if dotIndex > 0 {
				keyIsType = true
				key = keyPkg[dotIndex+1:]
				keyPkg = keyPkg[:dotIndex]
			}

			valuePkg = customType[strings.Index(customType, "]")+1:]

			isValueArr = strings.HasPrefix(valuePkg, "[]")
			if isValueArr {
				valuePkg = valuePkg[2:]
			}

			dotIndex = strings.LastIndex(valuePkg, ".")
			if dotIndex > 0 {
				valueIsType = true
				value = valuePkg[dotIndex+1:]
				valuePkg = valuePkg[:dotIndex]
			}

			found = true
		}
	}

	return
}

func (typ *Type) hasXGoSkipValidation(schema *openapi3.Schema) bool {
	return schema != nil && len(schema.Extensions) > 0 && schema.Extensions[goSkipValidation] != nil
}

func (typ *Type) getXGoSkipValidation(schema *openapi3.Schema) bool {
	var value = false

	if typ.hasXGoSkipValidation(schema) {
		if err := json.Unmarshal(schema.Extensions[goSkipValidation].(json.RawMessage), &value); err != nil {
			panic(err)
		}
	}

	return value
}

func (typ *Type) getXGoType(schema *openapi3.Schema) (string, string, bool) {
	if typ.hasXGoType(schema) {
		var customType string

		if err := json.Unmarshal(schema.Extensions[goType].(json.RawMessage), &customType); err != nil {
			panic(err)
		}

		index := strings.LastIndex(customType, ".")
		if index == -1 {
			return "", customType, true
		}

		return customType[:index], customType[index+1:], true
	}

	return "", "", false
}

func (typ *Type) getXGoPointer(schema *openapi3.Schema) bool {
	var value = false

	if typ.hasXGoPointer(schema) {
		if err := json.Unmarshal(schema.Extensions[goPointer].(json.RawMessage), &value); err != nil {
			panic(err)
		}
	}

	return value
}

func (typ *Type) hasXGoOmitempty(schema *openapi3.Schema) bool {
	if len(schema.Extensions) > 0 && schema.Extensions[goOmitempty] != nil {
		return true
	}

	return false
}

func (typ *Type) getXGoOmitempty(schema *openapi3.Schema) bool {
	var value = false

	if typ.hasXGoOmitempty(schema) {
		if err := json.Unmarshal(schema.Extensions[goOmitempty].(json.RawMessage), &value); err != nil {
			panic(err)
		}
	}

	return value
}

func (typ *Type) isCustomType(schema *openapi3.Schema) bool {
	return schema.Type == "string" && (schema.Format != "" || typ.hasXGoTypeStringParse(schema))
}

func (typ *Type) hasXGoSkipSecurityCheck(operation *openapi3.Operation) bool {
	return operation != nil && len(operation.Extensions) > 0 && operation.Extensions[goSkipSecurityCheck] != nil
}

func (typ *Type) getXGoSkipSecurityCheck(operation *openapi3.Operation) bool {
	var value = false

	if typ.hasXGoSkipSecurityCheck(operation) {
		switch v := operation.Extensions[goSkipSecurityCheck].(type) {
		case json.RawMessage:
			if err := json.Unmarshal(v, &value); err != nil {
				panic(err)
			}
		case bool:
			value = v
		default:
			panic(fmt.Sprintf("unexpected type for %s extension: %T", goSkipSecurityCheck, v))
		}
	}

	return value
}

func formatTagName(name string) string {
	if name == strings.ToUpper(name) {
		return strings.ToLower(name)
	}

	return strings.ToLower(name[:1]) + name[1:]
}
