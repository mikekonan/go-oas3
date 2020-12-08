package transformer

import (
	"strings"

	"github.com/ahmetb/go-linq"
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cast"
)

type InterfaceGenerator struct {
	generator  *Generator  `di.inject:"generator"`
	normalizer *Normalizer `di.inject:"normalizer"`
	filler     *TypeFiller `di.inject:"typeFiller"`
}

type operationResponse struct {
	ContentTypeBodyNameMap map[string]string
	Headers                map[string]*openapi3.HeaderRef
	StatusCode             string
}

type operationStruct struct {
	Tag                   string
	Name                  string
	RequestName           string
	ResponseName          string
	Responses             []operationResponse
	InterfaceResponseName string
	PrivateName           string
}

func (iGenerator *InterfaceGenerator) builders(swagger *openapi3.Swagger) (result jen.Code) {
	var builders []jen.Code

	linq.From(swagger.Paths).
		SelectManyT(func(kv linq.KeyValue) linq.Query {
			path := cast.ToString(kv.Key)
			var operationStructs []operationStruct

			linq.From(kv.Value.(*openapi3.PathItem).Operations()).
				SelectT(func(kv linq.KeyValue) operationStruct {
					name := iGenerator.normalizer.normalizeOperationName(path, cast.ToString(kv.Key))
					operation := kv.Value.(*openapi3.Operation)
					var operationResponses []operationResponse

					linq.From(operation.Responses).
						SelectT(func(kv linq.KeyValue) (response operationResponse) {
							response.ContentTypeBodyNameMap = map[string]string{}
							response.Headers = kv.Value.(*openapi3.ResponseRef).Value.Headers

							linq.From(kv.Value.(*openapi3.ResponseRef).Value.Content).
								ToMapByT(&response.ContentTypeBodyNameMap,
									func(kv linq.KeyValue) string { return cast.ToString(kv.Key) },
									func(kv linq.KeyValue) (structName string) {
										if "" == kv.Value.(*openapi3.MediaType).Schema.Ref {
											structName = name
											structName += strings.Title(iGenerator.normalizer.normalizeName(cast.ToString(kv.Key)))
											return structName
										}

										structName = iGenerator.normalizer.extractNameFromRef(kv.Value.(*openapi3.MediaType).Schema.Ref)
										return
									})

							response.StatusCode = cast.ToString(kv.Key)

							return
						}).ToSlice(&operationResponses)

					return operationStruct{
						Tag:                   operation.Tags[0],
						Name:                  name,
						PrivateName:           iGenerator.normalizer.decapitalize(name),
						RequestName:           name + "Request",
						InterfaceResponseName: name + "Response",
						ResponseName:          iGenerator.normalizer.decapitalize(name + "Response"),
						Responses:             operationResponses,
					}
				}).ToSlice(&operationStructs)

			return linq.From(operationStructs)
		}).
		SelectT(func(operationStruct operationStruct) jen.Code { return iGenerator.responseBuilders(operationStruct) }).
		ToSlice(&builders)

	return jen.Null().Add(builders...)
}

func (iGenerator *InterfaceGenerator) handlersTypes(swagger *openapi3.Swagger) jen.Code {
	var result []jen.Code

	linq.From(swagger.Paths).
		SelectT(func(kv linq.KeyValue) jen.Code {
			path := cast.ToString(kv.Key)
			var result []jen.Code

			linq.From(kv.Value.(*openapi3.PathItem).Operations()).
				SelectT(func(kv linq.KeyValue) jen.Code {
					name := iGenerator.normalizer.normalizeOperationName(path, cast.ToString(kv.Key))
					//operation := kv.Value.(*openapi3.Operation)

					result := iGenerator.normalizer.lineAfterEachCodeElement(iGenerator.responseType(name))

					return jen.Null().Add(result...)
				}).ToSlice(&result)

			result = iGenerator.normalizer.lineAfterEachCodeElement(result...)

			return jen.Null().Add(result...)
		}).ToSlice(&result)

	result = iGenerator.normalizer.lineAfterEachCodeElement(result...)
	return jen.Null().Add(result...)
}

func (iGenerator *InterfaceGenerator) handlersInterfaces(swagger *openapi3.Swagger) jen.Code {
	var result []jen.Code

	linq.From(swagger.Paths).
		SelectManyT(
			func(kv linq.KeyValue) linq.Query {
				path := cast.ToString(kv.Key)
				taggedInterfaceMethods := map[string][]jen.Code{}

				linq.From(kv.Value.(*openapi3.PathItem).Operations()).
					GroupByT(func(kv linq.KeyValue) string { return kv.Value.(*openapi3.Operation).Tags[0] },
						func(kv linq.KeyValue) []jen.Code {
							name := iGenerator.normalizer.normalizeOperationName(path, cast.ToString(kv.Key))
							operation := kv.Value.(*openapi3.Operation)

							if operation.RequestBody == nil {
								return []jen.Code{jen.Id(name).Params(jen.Id(name + "Request")).Params(jen.Id(name + "Response"))}
							}

							//if we have only one content type we dont need to have it inside function name
							if len(operation.RequestBody.Value.Content) == 1 {
								return []jen.Code{jen.Id(name).Params(jen.Id(name + "Request")).Params(jen.Id(name + "Response"))}
							}

							var contentTypedInterfaceMethods []jen.Code
							linq.From(operation.RequestBody.Value.Content).
								SelectT(func(kv linq.KeyValue) jen.Code {
									contentTypedName := name + iGenerator.normalizer.contentType(cast.ToString(kv.Key))
									return jen.Id(contentTypedName).Params(jen.Id(contentTypedName + "Request")).Params(jen.Id(name + "Response"))
								}).ToSlice(&contentTypedInterfaceMethods)

							return contentTypedInterfaceMethods
						}).
					ToMapByT(&taggedInterfaceMethods,
						func(kv linq.Group) interface{} { return kv.Key },
						func(kv linq.Group) (grouped []jen.Code) {
							linq.From(kv.Group).SelectMany(func(i interface{}) linq.Query { return linq.From(i) }).ToSlice(&grouped)
							return
						},
					)

				return linq.From(taggedInterfaceMethods)
			}).
		GroupByT(
			func(kv linq.KeyValue) interface{} { return kv.Key },
			func(kv linq.KeyValue) interface{} { return kv.Value },
		).
		SelectT(func(kv linq.Group) jen.Code {
			var grouped []jen.Code
			linq.From(kv.Group).SelectMany(func(i interface{}) linq.Query { return linq.From(i) }).ToSlice(&grouped)
			return jen.Type().Id(cast.ToString(kv.Key)).Interface(grouped...)
		}).
		ToSlice(&result)

	return jen.Null().Add(iGenerator.normalizer.lineAfterEachCodeElement(result...)...)
}

func (iGenerator *InterfaceGenerator) Generate(swagger *openapi3.Swagger) *jen.Statement {
	result := []jen.Code{
		iGenerator.responseStruct(),
		iGenerator.handlersTypes(swagger),
		iGenerator.builders(swagger),
		iGenerator.handlersInterfaces(swagger),
	}

	result = iGenerator.normalizer.lineAfterEachCodeElement(result...)

	return jen.Null().Add(result...)
}

func (iGenerator *InterfaceGenerator) responseStruct() jen.Code {
	return jen.Type().Id("response").Struct(
		jen.Id("statusCode").Id("int"),
		jen.Id("body").Interface(),
		jen.Id("contentType").Id("string"),
		jen.Id("headers").Map(jen.Id("string")).Id("string"),
	)
}

func (iGenerator *InterfaceGenerator) responseInterface(name string) jen.Code {
	name = iGenerator.normalizer.decapitalize(name)

	return jen.Type().Id(name + "Response").Interface(jen.Id(name + "Response").Params())
}

func (iGenerator *InterfaceGenerator) responseType(name string) jen.Code {
	decapicalizedName := iGenerator.normalizer.decapitalize(name)
	capitalizedName := strings.Title(name)

	interfaceDeclaration := jen.Type().Id(capitalizedName + "Response").Interface(jen.Id(decapicalizedName + "Response").Params())
	declaration := jen.Type().Id(decapicalizedName + "Response").Struct(jen.Id("response"))
	interfaceImplementation := jen.Func().Params(jen.Id(decapicalizedName + "Response")).Id(decapicalizedName + "Response").Params().Block()

	return jen.Null().Add(iGenerator.normalizer.lineAfterEachCodeElement(interfaceDeclaration, declaration, interfaceImplementation)...)
}

func (iGenerator *InterfaceGenerator) responseImplementationFunc(name string) jen.Code {
	return jen.Func().Params(jen.Id(strings.Title(name) + "Response")).Id(iGenerator.normalizer.decapitalize(name) + "Response").Params().Block()
}

//if hasHeaders && hasContentTypes
//N statusCode -> headersStruct -> M contentType -> body -> assemble

//if hasHeaders && !hasContentTypes
//N statusCode -> headersStruct -> assemble

//if !hasHeaders && hasContentTypes
//N statusCode -> M contentType -> body -> assemble

//if !hasHeaders && !hasContentTypes
//N statusCode -> assemble
func (iGenerator *InterfaceGenerator) responseBuilders(operationStruct operationStruct) jen.Code {
	builderConstructorName := iGenerator.builderConstructorName(operationStruct.Name)
	statusCodesBuilderName := iGenerator.statusCodesBuilderName(operationStruct.PrivateName)

	structBuilder := jen.Type().Id(statusCodesBuilderName).Struct(jen.Id("response"))
	structConstructor := jen.Func().Id(builderConstructorName).Params().Params(
		jen.Op("*").Id(statusCodesBuilderName)).Block(
		jen.Return().Id("new").Call(jen.Id(statusCodesBuilderName)),
	)

	var results []jen.Code

	linq.From(operationStruct.Responses).
		SelectT(func(resp operationResponse) (results []jen.Code) {
			hasHeaders := len(resp.Headers) > 0
			hasContentTypes := len(resp.ContentTypeBodyNameMap) > 0

			//OK
			if !hasHeaders && !hasContentTypes {
				//assembler struct
				assemblerName := iGenerator.assemblerName(operationStruct.Name + resp.StatusCode)
				results = append(results, jen.Type().Id(assemblerName).Struct(jen.Id("response")))

				//statusCode -> assembler
				results = append(results, jen.Func().Params(
					jen.Id("builder").Op("*").Id(statusCodesBuilderName)).Id("StatusCode"+resp.StatusCode).Params().Params(
					jen.Op("*").Id(assemblerName)).Block(
					jen.Id("builder").Dot("response").Dot("statusCode").Op("=").Lit(cast.ToInt(resp.StatusCode)),
					jen.Return().Op("&").Id(assemblerName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
				))

				//build
				results = append(results, jen.Func().Params(
					jen.Id("builder").Op("*").Id(assemblerName)).Id("Build").Params().Params(
					jen.Id(operationStruct.InterfaceResponseName)).Block(
					jen.Return().Id(operationStruct.ResponseName).Values(jen.Id("response").Op(":").Id("builder").Dot("response"))),
				)

				return
			}

			//OK
			if hasHeaders && !hasContentTypes {
				headersStructName := iGenerator.headersStructName(operationStruct.Name + resp.StatusCode)
				results = append(results, iGenerator.headersStruct(headersStructName, resp.Headers))
				headersBuilderName := iGenerator.headersBuilderName(operationStruct.PrivateName + resp.StatusCode)

				//headers struct
				results = append(results, iGenerator.headersStruct(headersStructName, resp.Headers))

				//statusCode -> headersStruct
				results = append(results, jen.Func().Params(
					jen.Id("builder").Op("*").Id(statusCodesBuilderName)).Id("StatusCode"+resp.StatusCode).Params().Params(
					jen.Op("*").Id(headersBuilderName)).Block(
					jen.Id("builder").Dot("response").Dot("statusCode").Op("=").Lit(cast.ToInt(resp.StatusCode)),
					jen.Return().Op("&").Id(headersBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
				))

				//headersStruct struct
				results = append(results, jen.Type().Id(headersBuilderName).Struct(jen.Id("response")))

				assemblerName := iGenerator.assemblerName(operationStruct.Name + resp.StatusCode)

				//headers -> assemble
				results = append(results,
					jen.Func().Params(
						jen.Id("builder").Op("*").Id(headersBuilderName)).Id("Headers").Params(
						jen.Id("headers").Id(headersStructName)).Params(
						jen.Op("*").Id(assemblerName)).Block(
						jen.Id("builder").Dot("headers").Op("=").Id("headers").Dot("toMap").Call(),
						jen.Return().Op("&").Id(assemblerName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
					))

				//assembler struct
				results = append(results, jen.Type().Id(assemblerName).Struct(jen.Id("response")))

				//assemble
				results = append(results, jen.Func().Params(
					jen.Id("builder").Op("*").Id(assemblerName)).Id("Build").Params().Params(
					jen.Id(operationStruct.InterfaceResponseName)).Block(
					jen.Return().Id(operationStruct.ResponseName).Values(jen.Id("response").Op(":").Id("builder").Dot("response"))),
				)

				return
			}

			if !hasHeaders && hasContentTypes {
				contentTypeBuilderName := iGenerator.contentTypeBuilderName(operationStruct.PrivateName + resp.StatusCode)

				//statusCode -> contentType
				results = append(results, jen.Func().Params(
					jen.Id("builder").Op("*").Id(statusCodesBuilderName)).Id("StatusCode"+resp.StatusCode).Params().Params(
					jen.Op("*").Id(contentTypeBuilderName)).Block(
					jen.Id("builder").Dot("response").Dot("statusCode").Op("=").Lit(cast.ToInt(resp.StatusCode)),
					jen.Return().Op("&").Id(contentTypeBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
				))

				//content-type struct
				results = append(results, jen.Type().Id(contentTypeBuilderName).Struct(jen.Id("response")))

				var contentTypeBodyBuild []jen.Code

				//content-types -> body -> build
				linq.From(resp.ContentTypeBodyNameMap).
					SelectT(func(kv linq.KeyValue) jen.Code {
						var result []jen.Code

						contentType := cast.ToString(kv.Key)
						contentTypeFuncName := iGenerator.contentTypeFuncName(contentType)
						bodyBuilderName := iGenerator.bodyGeneratorName(operationStruct.PrivateName+resp.StatusCode, contentType)

						//content-type -> body
						result = append(result, jen.Func().Params(
							jen.Id("builder").Op("*").Id(contentTypeBuilderName)).Id(contentTypeFuncName).Params().Params(
							jen.Op("*").Id(bodyBuilderName)).Block(
							jen.Id("builder").Dot("response").Dot("contentType").Op("=").Lit(contentType),
							jen.Return().Op("&").Id(bodyBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
						))

						//body struct
						result = append(result, jen.Type().Id(bodyBuilderName).Struct(jen.Id("response")))

						assemblerName := iGenerator.assemblerName(operationStruct.Name + resp.StatusCode + iGenerator.normalizer.contentType(contentType))

						//body builder
						result = append(result, jen.Func().Params(
							jen.Id("builder").Op("*").Id(bodyBuilderName)).Id("Body").Params(
							jen.Id("body").Id(cast.ToString(kv.Value))).Params(
							jen.Op("*").Id(assemblerName)).Block(
							jen.Id("builder").Dot("response").Dot("body").Op("=").Id("body"),
							jen.Return().Op("&").Id(assemblerName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
						))

						//assembler struct
						results = append(results, jen.Type().Id(assemblerName).Struct(jen.Id("response")))

						//assemble
						results = append(results, jen.Func().Params(
							jen.Id("builder").Op("*").Id(assemblerName)).Id("Build").Params().Params(
							jen.Id(operationStruct.InterfaceResponseName)).Block(
							jen.Return().Id(operationStruct.ResponseName).Values(jen.Id("response").Op(":").Id("builder").Dot("response"))),
						)

						return jen.Null().Add(iGenerator.normalizer.lineAfterEachCodeElement(result...)...)
					}).ToSlice(&contentTypeBodyBuild)

				results = iGenerator.normalizer.lineAfterEachCodeElement(append(results, contentTypeBodyBuild...)...)

				return
			}

			if hasHeaders && hasContentTypes {
				headersStructName := iGenerator.headersStructName(operationStruct.Name + resp.StatusCode)
				headersBuilderName := iGenerator.headersBuilderName(operationStruct.PrivateName + resp.StatusCode)

				//statusCode -> headers
				results = append(results, jen.Func().Params(
					jen.Id("builder").Op("*").Id(statusCodesBuilderName)).Id("StatusCode"+resp.StatusCode).Params().Params(
					jen.Op("*").Id(headersBuilderName)).Block(
					jen.Id("builder").Dot("response").Dot("statusCode").Op("=").Lit(cast.ToInt(resp.StatusCode)),
					jen.Return().Op("&").Id(headersBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
				))

				//headers struct
				results = append(results, iGenerator.headersStruct(headersStructName, resp.Headers))

				//headers builder struct
				results = append(results, jen.Type().Id(headersBuilderName).Struct(jen.Id("response")))

				//headers -> content-type
				contentTypeBuilderName := iGenerator.contentTypeBuilderName(operationStruct.PrivateName + resp.StatusCode)
				results = append(results,
					jen.Func().Params(
						jen.Id("builder").Op("*").Id(headersBuilderName)).Id("Headers").Params(
						jen.Id("headers").Id(headersStructName)).Params(
						jen.Op("*").Id(contentTypeBuilderName)).Block(
						jen.Id("builder").Dot("headers").Op("=").Id("headers").Dot("toMap").Call(),
						jen.Return().Op("&").Id(contentTypeBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
					))

				//content-type struct
				results = append(results, jen.Type().Id(contentTypeBuilderName).Struct(jen.Id("response")))

				var contentTypeBodyBuild []jen.Code

				//content-types -> body -> build
				linq.From(resp.ContentTypeBodyNameMap).
					SelectT(func(kv linq.KeyValue) jen.Code {
						var result []jen.Code

						contentType := cast.ToString(kv.Key)
						contentTypeFuncName := iGenerator.contentTypeFuncName(contentType)
						bodyBuilderName := iGenerator.bodyGeneratorName(operationStruct.PrivateName+resp.StatusCode, contentType)

						//content-type -> body
						result = append(result, jen.Func().Params(
							jen.Id("builder").Op("*").Id(contentTypeBuilderName)).Id(contentTypeFuncName).Params().Params(
							jen.Op("*").Id(bodyBuilderName)).Block(
							jen.Id("builder").Dot("response").Dot("contentType").Op("=").Lit(contentType),
							jen.Return().Op("&").Id(bodyBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
						))

						//body struct
						result = append(result, jen.Type().Id(bodyBuilderName).Struct(jen.Id("response")))

						assemblerName := iGenerator.assemblerName(operationStruct.Name + resp.StatusCode + iGenerator.normalizer.contentType(contentType))

						//body builder
						result = append(result, jen.Func().Params(
							jen.Id("builder").Op("*").Id(bodyBuilderName)).Id("Body").Params(
							jen.Id("body").Id(cast.ToString(kv.Value))).Params(
							jen.Op("*").Id(assemblerName)).Block(
							jen.Id("builder").Dot("response").Dot("body").Op("=").Id("body"),
							jen.Return().Op("&").Id(assemblerName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
						))

						//assembler struct
						results = append(results, jen.Type().Id(assemblerName).Struct(jen.Id("response")))

						//assemble
						results = append(results, jen.Func().Params(
							jen.Id("builder").Op("*").Id(assemblerName)).Id("Build").Params().Params(
							jen.Id(operationStruct.InterfaceResponseName)).Block(
							jen.Return().Id(operationStruct.ResponseName).Values(jen.Id("response").Op(":").Id("builder").Dot("response"))),
						)

						return jen.Null().Add(iGenerator.normalizer.lineAfterEachCodeElement(result...)...)
					}).ToSlice(&contentTypeBodyBuild)

				results = iGenerator.normalizer.lineAfterEachCodeElement(append(results, contentTypeBodyBuild...)...)
			}
			return
		}).
		SelectManyT(func(builders []jen.Code) linq.Query { return linq.From(builders) }).
		ToSlice(&results)

	return jen.Null().Add(iGenerator.normalizer.lineAfterEachCodeElement(append([]jen.Code{structBuilder, structConstructor}, results...)...)...)
}

func (iGenerator *InterfaceGenerator) headersStruct(name string, headers map[string]*openapi3.HeaderRef) jen.Code {
	if len(headers) == 0 {
		return jen.Null()
	}

	var headersCode []jen.Code

	linq.From(headers).SelectT(func(kv linq.KeyValue) jen.Code {
		name := iGenerator.normalizer.normalizeName(cast.ToString(kv.Key))
		field := jen.Id(name)

		iGenerator.filler.fillGoType(field, kv.Value.(*openapi3.HeaderRef).Value.Schema)

		return field
	}).ToSlice(&headersCode)

	headersStruct := jen.Type().Id(name).Struct(headersCode...)

	var headersMapCode []jen.Code

	linq.From(headers).SelectT(func(kv linq.KeyValue) jen.Code {
		name := iGenerator.normalizer.normalizeName(cast.ToString(kv.Key))
		return jen.Lit(name).Op(":").Id("cast").Dot("ToString").Call(jen.Id("headers").Dot(name))
	}).ToSlice(&headersMapCode)

	headersToMap := jen.Func().Params(
		jen.Id("headers").Id(name)).Id("toMap").Params().Params(
		jen.Map(jen.Id("string")).Id("string")).Block(
		jen.Return().Map(jen.Id("string")).Id("string").
			Values(headersMapCode...))

	return jen.Null().Add(iGenerator.normalizer.lineAfterEachCodeElement(headersStruct, headersToMap)...)
}

func (*InterfaceGenerator) builderConstructorName(name string) string {
	return name + "ResponseBuilder"
}

func (*InterfaceGenerator) statusCodesBuilderName(name string) string {
	return name + "StatusCodeResponseBuilder"
}

func (*InterfaceGenerator) headersBuilderName(name string) string {
	return name + "HeadersBuilder"
}

func (*InterfaceGenerator) headersStructName(name string) string {
	return name + "Headers"
}

func (*InterfaceGenerator) assemblerName(name string) string {
	return name + "ResponseBuilder"
}

func (iGenerator *InterfaceGenerator) contentTypeBuilderName(name string) string {
	return name + "ContentTypeBuilder"
}

func (iGenerator *InterfaceGenerator) contentTypeFuncName(contentType string) string {
	return iGenerator.normalizer.contentType(contentType)
}

func (iGenerator *InterfaceGenerator) bodyGeneratorName(name string, contentType string) string {
	return name + iGenerator.normalizer.contentType(contentType) + "BodyBuilder"
}
