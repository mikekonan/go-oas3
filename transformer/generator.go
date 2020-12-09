package transformer

import (
	"fmt"
	"strings"

	"github.com/ahmetb/go-linq"
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cast"
)

type Generator struct {
	normalizer *Normalizer `di.inject:"normalizer"`
	filler     *TypeFiller `di.inject:"typeFiller"`
}

func (generator *Generator) requestBody(name string, requestBody *openapi3.RequestBodyRef) (result jen.Code) {
	if requestBody == nil {
		return jen.Null()
	}

	requestBodyName := name + "RequestBody"
	result = jen.Type().Id(requestBodyName)

	if requestBody.Value != nil && len(requestBody.Value.Content) > 0 && requestBody.Value.Content["application/json"].Schema != nil {
		var schema = requestBody.Value.Content["application/json"].Schema
		generator.filler.fillGoType(result.(*jen.Statement), requestBodyName, schema)
	}

	return
}

func (generator *Generator) responsesBodies(name string, suffix string, responsesBody map[string]*openapi3.ResponseRef) (result []jen.Code) {
	linq.From(responsesBody).
		SelectT(func(kv linq.KeyValue) jen.Code {
			responseBody := kv.Value.(*openapi3.ResponseRef)
			if responseBody == nil || responseBody.Value == nil || len(responseBody.Value.Content) == 0 || responseBody.Value.Content["application/json"].Schema == nil {
				return jen.Null()
			}

			bodyName := name + cast.ToString(kv.Key) + suffix
			result := jen.Type().Id(bodyName)
			var schema = responseBody.Value.Content["application/json"].Schema
			generator.filler.fillGoType(result, bodyName, schema)

			return result
		}).
		ToSlice(&result)

	return
}

func (generator *Generator) requestParameters(paths map[string]*openapi3.PathItem) (results map[string][]jen.Code) {
	results = map[string][]jen.Code{}

	linq.From(paths).
		SelectManyT(func(kv linq.KeyValue) linq.Query {
			path := cast.ToString(kv.Key)
			operationsCodeTags := map[string][]jen.Code{}

			linq.From(kv.Value.(*openapi3.PathItem).Operations()).
				GroupByT(
					func(kv linq.KeyValue) string { return kv.Value.(*openapi3.Operation).Tags[0] },
					func(kv linq.KeyValue) (result []jen.Code) {
						name := generator.normalizer.normalizeOperationName(path, cast.ToString(kv.Key))
						operation := kv.Value.(*openapi3.Operation)
						if operation.RequestBody == nil {
							result = append(result, generator.requestParameterStruct(name, "", operation))
							return
						}

						if operation.RequestBody != nil && len(operation.RequestBody.Value.Content) == 1 {
							result = append(result, generator.requestParameterStruct(name, "", operation))
							return
						}

						var contentTypeResult []jen.Code
						linq.From(operation.RequestBody.Value.Content).
							SelectT(func(kv linq.KeyValue) jen.Code { return generator.requestParameterStruct(name, cast.ToString(kv.Key), operation) }).
							ToSlice(&contentTypeResult)

						result = append(result, contentTypeResult...)

						result = generator.normalizer.doubleLineAfterEachElement(result...)

						return
					},
				).
				ToMapByT(&operationsCodeTags,
					func(kv linq.Group) interface{} { return kv.Key },
					func(kv linq.Group) (grouped []jen.Code) {
						linq.From(kv.Group).SelectMany(func(i interface{}) linq.Query { return linq.From(i) }).ToSlice(&grouped)
						return
					},
				)

			return linq.From(operationsCodeTags)
		}).
		GroupByT(
			func(kv linq.KeyValue) interface{} { return kv.Key },
			func(kv linq.KeyValue) interface{} { return kv.Value },
		).
		ToMapByT(&results,
			func(kv linq.Group) interface{} { return kv.Key },
			func(kv linq.Group) (grouped []jen.Code) {
				linq.From(kv.Group).SelectMany(func(i interface{}) linq.Query { return linq.From(i) }).ToSlice(&grouped)
				return
			},
		)

	return
}

func (generator *Generator) components(components map[string]*openapi3.SchemaRef) (result map[string]jen.Code) {
	result = map[string]jen.Code{}

	linq.From(components).
		WhereT(func(kv linq.KeyValue) bool { return len(kv.Value.(*openapi3.SchemaRef).Value.Enum) == 0 }). //filter enums
		ToMapByT(&result,
			func(kv linq.KeyValue) string { return cast.ToString(kv.Key) },
			func(kv linq.KeyValue) jen.Code {
				schemaRef := kv.Value.(*openapi3.SchemaRef)
				return generator.objectFromSchema(cast.ToString(kv.Key), schemaRef)
			},
		)

	return
}

func (generator *Generator) requestParameterStruct(name string, contentType string, operation *openapi3.Operation) jen.Code {
	type parameter struct {
		In   string
		Code jen.Code
	}

	var additionalParameters []parameter

	if contentType != "" {
		name += generator.normalizer.contentType(contentType)
		bodyTypeName := generator.normalizer.extractNameFromRef(operation.RequestBody.Value.Content[contentType].Schema.Ref)
		if bodyTypeName == "" {
			bodyTypeName = name + "RequestBody"
		}

		additionalParameters = append(additionalParameters, parameter{In: "Body", Code: jen.Id("Body").Id(bodyTypeName)})
	}

	var (
		requestType = jen.Type().Id(name + "Request")
		parameters  []jen.Code
	)

	linq.From(operation.Parameters).
		GroupByT(
			func(parameter *openapi3.ParameterRef) string { return parameter.Value.In },
			func(parameter *openapi3.ParameterRef) *openapi3.ParameterRef { return parameter }).
		SelectT(
			func(group linq.Group) (parameter parameter) {
				var structFields []jen.Code
				linq.From(group.Group).
					OrderByT(func(parameter *openapi3.ParameterRef) string { return parameter.Value.Name }).
					SelectT(func(parameter *openapi3.ParameterRef) (result jen.Code) {
						name := generator.normalizer.normalizeName(parameter.Value.Name)
						var statement = jen.Id(name)
						generator.filler.fillGoType(statement, name, parameter.Value.Schema)
						return statement
					}).
					ToSlice(&structFields)

				parameter.In = cast.ToString(group.Key)
				parameter.Code = jen.Id(generator.normalizer.normalizeName(cast.ToString(group.Key))).Struct(structFields...)

				return
			}).
		Concat(linq.From(additionalParameters)).
		OrderByT(func(parameter parameter) string { return parameter.In }).
		SelectT(func(parameter parameter) jen.Code { return parameter.Code }).
		ToSlice(&parameters)

	return requestType.Struct(parameters...)
}

func (generator *Generator) enumFromSchema(name string, schema *openapi3.SchemaRef) jen.Code {
	if len(schema.Ref) > 0 {
		return jen.Null()
	}

	var result []jen.Code
	var enumValues []jen.Code

	result = append(result, jen.Type().Id(generator.normalizer.normalizeName(name)).String())

	linq.From(schema.Value.Enum).SelectT(func(value string) jen.Code {
		return jen.Var().Id(name + generator.normalizer.normalizeName(strings.Title(value))).Id(name).Op("=").Lit(value)
	}).ToSlice(&enumValues)

	var enumSwitchCases []jen.Code

	linq.From(schema.Value.Enum).SelectT(func(value string) jen.Code {
		return jen.Id(name + generator.normalizer.normalizeName(strings.Title(value)))
	}).ToSlice(&enumSwitchCases)

	result = append(result, enumValues...)

	result = append(result, jen.Func().Params(
		jen.Id("enum").Op("*").Id(name)).Id("UnmarshalJSON").Params(
		jen.Id("data").Index().Id("byte")).Params(
		jen.Id("error")).Block(
		jen.Var().Id("strValue").Id("string"),
		jen.If(jen.Id("err").Op(":=").Qual("encoding/json",
			"Unmarshal").Call(jen.Id("data"),
			jen.Op("&").Id("strValue")),
			jen.Id("err").Op("!=").Id("nil")).Block(
			jen.Return().Id("err")),
		jen.Id("enumValue").Op(":=").Id(name).Call(jen.Id("strValue")),
		jen.Switch(jen.Id("enumValue")).Block(
			jen.Case(enumSwitchCases...).Block(
				jen.Op("*").Id("enum").Op("=").Id("enumValue"),
				jen.Return().Id("nil"))),
		jen.Return().Qual("fmt",
			"Errorf").Call(jen.Lit(fmt.Sprintf("could not unmarshal %s", name))),
	))

	result = generator.normalizer.lineAfterEachElement(result...)

	return jen.Null().Add(result...)
}
func (generator *Generator) objectFromSchema(name string, schema *openapi3.SchemaRef) *jen.Statement {
	name = generator.normalizer.normalizeName(name)
	typeDeclaration := jen.Type().Id(name)

	if len(schema.Value.Properties) == 0 {
		if len(schema.Value.Enum) > 0 {
			generator.filler.fillGoType(typeDeclaration, name+"Enum", schema)
			return typeDeclaration
		}

		typeDeclaration.Interface()
		return typeDeclaration
	}

	return typeDeclaration.Struct(generator.typeProperties(name, schema.Value)...)
}

func (generator *Generator) typeProperties(typeName string, schema *openapi3.Schema) (parameters []jen.Code) {
	linq.From(schema.Properties).
		OrderByT(func(kv linq.KeyValue) interface{} { return kv.Key }).
		SelectT(func(kv linq.KeyValue) interface{} {
			originName := cast.ToString(kv.Key)
			name := generator.normalizer.normalizeName(originName)
			parameter := jen.Id(name)
			schemaRef := kv.Value.(*openapi3.SchemaRef)
			if len(schemaRef.Value.Enum) > 0 {
				name = typeName + strings.Title(name) + "Enum"
			}

			generator.filler.fillGoType(parameter, name, schemaRef)
			generator.filler.fillJsonTag(parameter, originName)
			return parameter
		}).ToSlice(&parameters)

	return
}

func (generator *Generator) componentsFromPaths(paths openapi3.Paths) (result map[string]jen.Code) {
	result = map[string]jen.Code{}

	linq.From(paths).
		SelectManyT(func(kv linq.KeyValue) linq.Query {
			path := cast.ToString(kv.Key)
			componentsByName := map[string]jen.Code{}

			linq.From(kv.Value.(*openapi3.PathItem).Operations()).
				WhereT(func(kv linq.KeyValue) bool {
					operation := kv.Value.(*openapi3.Operation)
					return operation.RequestBody != nil && len(operation.RequestBody.Value.Content) > 0 &&
						linq.From(operation.RequestBody.Value.Content).
							AnyWithT(func(kv linq.KeyValue) bool { return kv.Value.(*openapi3.MediaType).Schema.Ref == "" })
				}).
				SelectManyT(
					func(kv linq.KeyValue) linq.Query {
						result := map[string]jen.Code{}
						name := generator.normalizer.normalizeOperationName(path, cast.ToString(kv.Key))
						operation := kv.Value.(*openapi3.Operation)

						linq.From(operation.RequestBody.Value.Content).
							ToMapByT(&result,
								func(kv linq.KeyValue) string { return name + generator.normalizer.contentType(cast.ToString(kv.Key)+"RequestBody") },
								func(kv linq.KeyValue) jen.Code {
									meType := kv.Value.(*openapi3.MediaType)

									if kv.Value.(*openapi3.MediaType).Schema.Ref == "" {
										objName := name + generator.normalizer.contentType(cast.ToString(kv.Key)+"RequestBody")
										return generator.objectFromSchema(objName, meType.Schema)
									}

									return jen.Null()
								})

						return linq.From(result)
					},
				).
				ToMapByT(&componentsByName,
					func(kv linq.KeyValue) interface{} { return kv.Key },
					func(kv linq.KeyValue) interface{} { return kv.Value })

			return linq.From(componentsByName)
		}).
		GroupByT(
			func(kv linq.KeyValue) interface{} { return kv.Key },
			func(kv linq.KeyValue) interface{} { return kv.Value },
		).
		ToMapByT(&result,
			func(kv linq.Group) interface{} { return kv.Key },
			func(kv linq.Group) jen.Code {
				var grouped []jen.Code
				linq.From(kv.Group).ToSlice(&grouped)
				return jen.Add(generator.normalizer.doubleLineAfterEachElement(grouped...)...)
			},
		)

	return
}

func (generator *Generator) enums(swagger *openapi3.Swagger) jen.Code {
	var pathsResult []jen.Code

	linq.From(swagger.Paths).
		SelectManyT(func(kv linq.KeyValue) linq.Query {
			var result []jen.Code
			path := cast.ToString(kv.Key)

			linq.From(kv.Value.(*openapi3.PathItem).Operations()).
				SelectManyT(func(kv linq.KeyValue) linq.Query {
					var requestBodyResults []jen.Code

					name := generator.normalizer.normalizeOperationName(path, cast.ToString(kv.Key))
					operation := kv.Value.(*openapi3.Operation)

					if operation.RequestBody != nil {
						linq.From(operation.RequestBody.Value.Content).
							SelectT(func(kv linq.KeyValue) jen.Code {
								schema := kv.Value.(*openapi3.MediaType).Schema

								namePrefix := generator.normalizer.normalizeName(name + generator.normalizer.contentType(cast.ToString(kv.Key)))

								if len(schema.Value.Enum) > 0 {
									return generator.enumFromSchema(namePrefix+"RequestBodyEnum", schema)
								}

								var result []jen.Code
								linq.From(schema.Value.Properties).WhereT(func(kv linq.KeyValue) bool {
									return len(kv.Value.(*openapi3.SchemaRef).Value.Enum) > 0
								}).SelectT(func(kv linq.KeyValue) interface{} {
									enumName := namePrefix + generator.normalizer.normalizeName(strings.Title(cast.ToString(kv.Key))) + "Enum"
									enumName = generator.normalizer.normalizeName(enumName)
									return generator.enumFromSchema(enumName, kv.Value.(*openapi3.SchemaRef))
								}).ToSlice(&result)

								return jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(result...)...)
							}).ToSlice(&requestBodyResults)
					}

					var result []jen.Code
					linq.From(operation.Responses).
						SelectManyT(func(kv linq.KeyValue) linq.Query {
							return linq.From(kv.Value.(*openapi3.ResponseRef).Value.Content).
								SelectT(func(kv linq.KeyValue) jen.Code {
									schema := kv.Value.(*openapi3.MediaType).Schema
									namePrefix := generator.normalizer.normalizeName(name + generator.normalizer.contentType(cast.ToString(kv.Key)))

									if len(schema.Value.Enum) > 0 {
										return generator.enumFromSchema(namePrefix+"ResponseBodyEnum", schema)
									}

									var result []jen.Code
									linq.From(schema.Value.Properties).WhereT(func(kv linq.KeyValue) bool {
										return len(kv.Value.(*openapi3.SchemaRef).Value.Enum) > 0
									}).SelectT(func(kv linq.KeyValue) interface{} {
										enumName := namePrefix + generator.normalizer.normalizeName(strings.Title(cast.ToString(kv.Key))) + "Enum"
										enumName = generator.normalizer.normalizeName(enumName)
										return generator.enumFromSchema(enumName, kv.Value.(*openapi3.SchemaRef))
									}).ToSlice(&result)

									return jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(result...)...)
								})
						}).
						Concat(linq.From(requestBodyResults)).
						ToSlice(&result)

					return linq.From(result)
				}).ToSlice(&result)

			return linq.From(result)
		}).ToSlice(&pathsResult)

	var componentsResult []jen.Code

	linq.From(swagger.Components.Schemas).
		SelectT(func(kv linq.KeyValue) jen.Code {
			namePrefix := generator.normalizer.normalizeName(cast.ToString(kv.Key))
			schema := kv.Value.(*openapi3.SchemaRef)

			if len(schema.Value.Enum) > 0 {
				return generator.enumFromSchema(namePrefix, schema)
			}

			var result []jen.Code
			linq.From(schema.Value.Properties).WhereT(func(kv linq.KeyValue) bool {
				return len(kv.Value.(*openapi3.SchemaRef).Value.Enum) > 0
			}).SelectT(func(kv linq.KeyValue) interface{} {
				enumName := namePrefix + generator.normalizer.normalizeName(strings.Title(cast.ToString(kv.Key))) + "Enum"
				enumName = generator.normalizer.normalizeName(enumName)
				return generator.enumFromSchema(enumName, kv.Value.(*openapi3.SchemaRef))
			}).ToSlice(&result)

			return jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(result...)...)
		}).ToSlice(&componentsResult)

	return jen.Null().Add(generator.normalizer.lineAfterEachElement(pathsResult...)...).Add(generator.normalizer.lineAfterEachElement(componentsResult...)...)
}
