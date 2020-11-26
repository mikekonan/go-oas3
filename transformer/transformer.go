package transformer

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/ahmetb/go-linq"
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cast"

	"github.com/mikekonan/go-oas3/configurator"
)

type Transformer struct {
	config *configurator.Config `di.inject:"config"`
}

func (transformer *Transformer) Transform(swagger *openapi3.Swagger) {
	components := transformer.generateComponents(swagger.Components.Schemas)
	pathsComponents := transformer.generatePathsComponents(swagger.Paths)

	keka := pathsComponents["Callbacks"]
	kek1 := keka[0]
	kek2 := keka[1]

	lel1 := jen.NewFile("kek")
	lel1.Add(kek1)
	lel1.Add(kek2)
	lel2 := lel1.GoString()
	fmt.Println(pathsComponents, components, kek1, kek2, lel2)
}

func (transformer *Transformer) generatePathsComponents(paths map[string]*openapi3.PathItem) (results map[string][]jen.Code) {
	results = map[string][]jen.Code{}

	linq.From(paths).
		SelectManyT(func(kv linq.KeyValue) linq.Query {
			path := cast.ToString(kv.Key)
			operationsCodeTags := map[string][]jen.Code{}

			linq.From(kv.Value.(*openapi3.PathItem).Operations()).ToMapByT(
				&operationsCodeTags,
				func(kv linq.KeyValue) string { return kv.Value.(*openapi3.Operation).Tags[0] },
				func(kv linq.KeyValue) []jen.Code {
					name := transformer.createOperationID(path, cast.ToString(kv.Key))
					operation := kv.Value.(*openapi3.Operation)
					operationRequestParams := transformer.generateRequestParams(name+"RequestParams", operation.Parameters)
					requestBody := transformer.generateRequestBody(name+"RequestBody", operation.RequestBody)

					return []jen.Code{operationRequestParams, requestBody}
				})

			return linq.From(operationsCodeTags)
		}).
		GroupByT(
			func(kv linq.KeyValue) interface{} { return kv.Key },
			func(kv linq.KeyValue) interface{} { return kv.Value },
		).
		ToMapByT(&results,
			func(kv linq.Group) interface{} {
				return kv.Key
			},
			func(kv linq.Group) (grouped []jen.Code) {
				linq.From(kv.Group).SelectMany(func(i interface{}) linq.Query { return linq.From(i) }).ToSlice(&grouped)
				return
			},
		)

	return
}

func (transformer *Transformer) generateComponents(components map[string]*openapi3.SchemaRef) (result map[string]jen.Code) {
	result = map[string]jen.Code{}

	linq.From(components).
		ToMapByT(&result,
			func(kv linq.KeyValue) string { return cast.ToString(kv.Key) },
			func(kv linq.KeyValue) jen.Code {
				value := kv.Value.(*openapi3.SchemaRef)
				return transformer.generateObject(cast.ToString(kv.Key), value.Value)
			},
		)

	return
}

func (transformer *Transformer) generateRequestParams(name string, parameters openapi3.Parameters) jen.Code {
	var (
		requestParams      = jen.Type().Id(name)
		requestParamsParts []jen.Code
	)

	linq.From(parameters).
		GroupByT(
			func(parameter *openapi3.ParameterRef) string { return parameter.Value.In },
			func(parameter *openapi3.ParameterRef) *openapi3.ParameterRef { return parameter }).
		SelectT(
			func(group linq.Group) (field jen.Code) {
				var structFields []jen.Code
				linq.From(group.Group).
					OrderByT(func(parameter *openapi3.ParameterRef) string { return parameter.Value.Name }).
					SelectT(func(parameter *openapi3.ParameterRef) (result jen.Code) {
						var statement = jen.Id(transformer.normalizeName(parameter.Value.Name))
						transformer.fillGoType(statement, parameter.Value.Schema)
						return statement
					}).
					ToSlice(&structFields)

				return jen.Id(transformer.normalizeName(cast.ToString(group.Key))).Struct(structFields...)
			}).
		ToSlice(&requestParamsParts)

	return requestParams.Struct(requestParamsParts...)
}

func (transformer *Transformer) normalizeName(str string) string {
	separators := "-#@!$&=.+:;_~ (){}[]"
	s := strings.Trim(str, " ")

	n := ""
	capNext := true
	for _, v := range s {
		if unicode.IsUpper(v) {
			n += string(v)
		}
		if unicode.IsDigit(v) {
			n += string(v)
		}
		if unicode.IsLower(v) {
			if capNext {
				n += strings.ToUpper(string(v))
			} else {
				n += string(v)
			}
		}

		if strings.ContainsRune(separators, v) {
			capNext = true
		} else {
			capNext = false
		}
	}

	if len(n) > 3 {
		if strings.ToLower(n[len(n)-4:]) == "uuid" {
			n = n[:len(n)-4] + "UUID"
		}
	}

	if len(n) > 1 {
		if strings.ToLower(n[len(n)-2:]) == "id" {
			n = n[:len(n)-2] + "ID"
		}
	}

	return n
}

func (transformer *Transformer) generateObject(name string, schema *openapi3.Schema) *jen.Statement {
	return jen.Type().Id(transformer.normalizeName(name)).Struct(transformer.generateGoTypeProperties(schema)...)
}

func (transformer *Transformer) generateGoTypeProperties(schema *openapi3.Schema) (parameters []jen.Code) {
	linq.From(schema.Properties).
		OrderByT(func(kv linq.KeyValue) interface{} { return kv.Key }).
		SelectT(func(kv linq.KeyValue) interface{} {
			parameter := jen.Id(transformer.normalizeName(cast.ToString(kv.Key)))
			schemaRef := kv.Value.(*openapi3.SchemaRef)
			transformer.fillGoType(parameter, schemaRef)

			return parameter
		}).ToSlice(&parameters)

	return
}

func (transformer *Transformer) fillGoType(into *jen.Statement, schemaRef *openapi3.SchemaRef) {
	schema := schemaRef.Value

	if schema.AnyOf != nil || schema.OneOf != nil || schema.AllOf != nil {
		into.Interface()
		return
	}

	switch schema.Type {
	case "object":
		if schemaRef.Ref != "" {
			typeName := transformer.normalizeName(schemaRef.Ref[strings.LastIndex(schemaRef.Ref, "/")+1:])
			into.Id(typeName)
			return
		}

		if len(schema.Properties) == 0 {
			into.Interface()
			return
		}
		return
	case "array":
		//
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

func (transformer *Transformer) createOperationID(path string, method string) string {
	return transformer.normalizeName(strings.ReplaceAll(strings.ToLower(method)+path, "/", "-"))
}

func (transformer *Transformer) generateRequestBody(name string, requestBody *openapi3.RequestBodyRef) (result jen.Code) {
	if requestBody == nil {
		return jen.Null()
	}

	result = jen.Type().Id(name)

	if requestBody.Value != nil && len(requestBody.Value.Content) > 0 && requestBody.Value.Content["application/json"].Schema != nil {
		var schema = requestBody.Value.Content["application/json"].Schema
		transformer.fillGoType(result.(*jen.Statement), schema)
		fmt.Println(schema)
	}

	return
}

//linq.From(operations).
//	SelectManyT(func(kv linq.KeyValue) linq.Query {
//		name := transformer.createOperationID(path, cast.ToString(kv.Key))
//		operation := kv.Value.(*openapi3.Operation)
//		operationRequestParams := transformer.generateRequestParams(name+"RequestParams", operation.Parameters)
//		requestBody := transformer.generateRequestBody(name+"RequestBody", operation.RequestBody)
//
//		return linq.From([]jen.Code{operationRequestParams, requestBody})
//	}).ToSlice(&pathsCode)
