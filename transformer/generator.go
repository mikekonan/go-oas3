package transformer

import (
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

	result = jen.Type().Id(name + "RequestBody")

	if requestBody.Value != nil && len(requestBody.Value.Content) > 0 && requestBody.Value.Content["application/json"].Schema != nil {
		var schema = requestBody.Value.Content["application/json"].Schema
		generator.filler.fillGoType(result.(*jen.Statement), schema)
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

			result := jen.Type().Id(name + cast.ToString(kv.Key) + suffix)
			var schema = responseBody.Value.Content["application/json"].Schema
			generator.filler.fillGoType(result, schema)

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

						result = generator.normalizer.lineAfterEachCodeElement(result...)

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
		ToMapByT(&result,
			func(kv linq.KeyValue) string { return cast.ToString(kv.Key) },
			func(kv linq.KeyValue) jen.Code {
				value := kv.Value.(*openapi3.SchemaRef)
				return generator.objectFromSchema(cast.ToString(kv.Key), value.Value)
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
						var statement = jen.Id(generator.normalizer.normalizeName(parameter.Value.Name))
						generator.filler.fillGoType(statement, parameter.Value.Schema)
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

func (generator *Generator) objectFromSchema(name string, schema *openapi3.Schema) *jen.Statement {
	typeDeclaration := jen.Type().Id(generator.normalizer.normalizeName(name))

	if len(schema.Properties) == 0 {
		typeDeclaration.Interface()
		return typeDeclaration
	}

	return typeDeclaration.Struct(generator.typeProperties(schema)...)
}

func (generator *Generator) typeProperties(schema *openapi3.Schema) (parameters []jen.Code) {
	linq.From(schema.Properties).
		OrderByT(func(kv linq.KeyValue) interface{} { return kv.Key }).
		SelectT(func(kv linq.KeyValue) interface{} {
			originName := cast.ToString(kv.Key)
			name := generator.normalizer.normalizeName(originName)
			parameter := jen.Id(name)
			schemaRef := kv.Value.(*openapi3.SchemaRef)
			generator.filler.fillGoType(parameter, schemaRef)
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
										return generator.objectFromSchema(objName, meType.Schema.Value)
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
				return jen.Add(generator.normalizer.lineAfterEachCodeElement(grouped...)...)
			},
		)

	return
}
