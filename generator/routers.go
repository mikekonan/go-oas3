package generator

import (
	"strings"

	"github.com/ahmetb/go-linq"
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cast"
)

// groupedOperations represents operations grouped by tag
type groupedOperations struct {
	tag        interface{}
	operations []operationWithPath
}

// operationWithPath represents an operation with its HTTP method and path
type operationWithPath struct {
	method    interface{}
	path      string
	operation *openapi3.Operation
}

// wrappers generates wrapper functions for all operations
func (generator *Generator) wrappers(swagger *openapi3.T) jen.Code {
	var results []jen.Code

	linq.From(generator.groupedOperations(swagger)).
		SelectT(func(groupedOperations groupedOperations) jen.Code {
			tag := generator.normalizer.normalize(cast.ToString(groupedOperations.tag))

			var routes []jen.Code
			linq.From(groupedOperations.operations).
				SelectT(func(operation operationWithPath) jen.Code {
					method := generator.normalizer.normalize(strings.Title(strings.ToLower(cast.ToString(operation.method))))

					if operation.operation.RequestBody == nil || len(operation.operation.RequestBody.Value.Content) == 1 {
						name := generator.normalizer.normalizeOperationName(operation.path, cast.ToString(operation.method))
						return jen.Id("router").Dot("router").Dot(method).Call(jen.Lit(operation.path), jen.Id("router").Dot(name))
					}

					var result []jen.Code
					linq.From(operation.operation.RequestBody.Value.Content).
						SelectT(func(kv linq.KeyValue) jen.Code {
							name := generator.normalizer.normalizeOperationName(operation.path, cast.ToString(operation.method)) + generator.normalizer.contentType(cast.ToString(kv.Key))
							return jen.Id("router").Dot("router").Dot(method).Call(jen.Lit(operation.path), jen.Id("router").Dot(name))
						}).ToSlice(&result)

					return jen.Add(generator.normalizer.lineAfterEachElement(result...)...)
				}).ToSlice(&routes)

			var wrappers []jen.Code

			linq.From(groupedOperations.operations).
				SelectManyT(func(operation operationWithPath) linq.Query {
					name := generator.normalizer.normalizeOperationName(operation.path, cast.ToString(operation.method))

					if operation.operation.RequestBody == nil {
						return linq.From([]jen.Code{generator.wrapper(name, name+SuffixRequest, tag, cast.ToString(operation.method), operation.path, operation.operation, nil, "")})
					}

					if len(operation.operation.RequestBody.Value.Content) == 1 {
						contentType := cast.ToString(linq.From(operation.operation.RequestBody.Value.Content).SelectT(func(kv linq.KeyValue) string { return cast.ToString(kv.Key) }).First())
						mediaType := linq.From(operation.operation.RequestBody.Value.Content).SelectT(func(kv linq.KeyValue) *openapi3.MediaType { return kv.Value.(*openapi3.MediaType) }).First().(*openapi3.MediaType)

						return linq.From([]jen.Code{generator.wrapper(name, name+SuffixRequest, tag, cast.ToString(operation.method), operation.path, operation.operation, mediaType.Schema, contentType)})
					}

					var result []jen.Code
					linq.From(operation.operation.RequestBody.Value.Content).
						SelectT(func(kv linq.KeyValue) jen.Code {
							contentType := cast.ToString(kv.Key)
							name := generator.normalizer.normalizeOperationName(operation.path, cast.ToString(operation.method)) + generator.normalizer.contentType(contentType)
							mediaType := kv.Value.(*openapi3.MediaType)

							return generator.wrapper(name, name+SuffixRequest, tag, cast.ToString(operation.method), operation.path, operation.operation, mediaType.Schema, contentType)
						}).ToSlice(&result)

					return linq.From(result)
				}).ToSlice(&wrappers)

			wrappers = generator.normalizer.doubleLineAfterEachElement(wrappers...)

			hasSecuritySchemas := swagger.Components != nil && len(swagger.Components.SecuritySchemes) > 0
			return jen.Add(
				generator.handler(tag, tag+"Service", tag+"Router", hasSecuritySchemas, groupedOperations.operations),
				jen.Line(), jen.Line(),
				generator.router(tag+"Router", tag+"Service", hasSecuritySchemas),
				jen.Line(), jen.Line(),
			).Add(wrappers...)
		}).ToSlice(&results)

	results = generator.normalizer.doubleLineAfterEachElement(results...)
	return jen.Add(results...)
}

// wrapper generates a wrapper function for a specific operation
func (generator *Generator) wrapper(name string, requestName string, routerName, method string, path string, operation *openapi3.Operation, requestBody *openapi3.SchemaRef, contentType string) jen.Code {
	wrapperName := generator.normalizer.decapitalize(name)

	hookStruct := generator.hooksStruct()
	processingResultType := generator.requestProcessingResultType()

	parsers := generator.wrapperRequestParsers(wrapperName, operation)
	bodyParser := generator.wrapperBody(method, path, contentType, wrapperName, operation, requestBody)
	securityParser := generator.wrapperSecurity(name, operation)
	requestParser := generator.wrapperRequestParser(name, requestName, routerName, method, path, operation, requestBody, contentType)

	wrapperBody := []jen.Code{
		jen.Var().Id("request").Qual(generator.config.Package, requestName),
		jen.Line(),
	}

	// Add parsers
	if len(parsers) > 0 {
		wrapperBody = append(wrapperBody, parsers...)
		wrapperBody = append(wrapperBody, jen.Line())
	}

	// Add body parser
	if bodyParser != jen.Null() {
		wrapperBody = append(wrapperBody, bodyParser)
		wrapperBody = append(wrapperBody, jen.Line())
	}

	// Add security parser
	if securityParser != jen.Null() {
		wrapperBody = append(wrapperBody, securityParser)
		wrapperBody = append(wrapperBody, jen.Line())
	}

	// Add request parser
	if requestParser != jen.Null() {
		wrapperBody = append(wrapperBody, requestParser)
		wrapperBody = append(wrapperBody, jen.Line())
	}

	// Add service call
	wrapperBody = append(wrapperBody,
		jen.Id("response").Op(":=").Id("router").Dot("service").Dot(name).Call(jen.Id("r").Dot("Context").Call(), jen.Op("&").Id("request")),
		jen.Id("response").Dot("WriteTo").Call(jen.Id("w")),
	)

	return jen.Add(
		hookStruct,
		jen.Line(), jen.Line(),
		processingResultType,
		jen.Line(), jen.Line(),
		jen.Func().Params(jen.Id("router").Op("*").Id(routerName)).Id(wrapperName).Params(
			jen.Id("w").Qual(PackageNetHTTP, "ResponseWriter"),
			jen.Id("r").Op("*").Qual(PackageNetHTTP, "Request"),
		).Block(wrapperBody...),
	)
}

// groupedOperations groups operations by their tags
func (generator *Generator) groupedOperations(swagger *openapi3.T) []groupedOperations {
	var result []groupedOperations

	linq.From(swagger.Paths.Map()).
		SelectManyT(func(kv linq.KeyValue) linq.Query {
			path := cast.ToString(kv.Key)
			return linq.From(kv.Value.(*openapi3.PathItem).Operations()).
				SelectT(func(kv linq.KeyValue) operationWithPath {
					return operationWithPath{
						method:    kv.Key,
						path:      path,
						operation: kv.Value.(*openapi3.Operation),
					}
				})
		}).
		GroupByT(
			func(operation operationWithPath) string {
				if len(operation.operation.Tags) > 0 {
					return operation.operation.Tags[0]
				}
				return "default"
			},
			func(operation operationWithPath) operationWithPath { return operation },
		).
		SelectT(func(group linq.Group) groupedOperations {
			var operations []operationWithPath
			linq.From(group.Group).ToSlice(&operations)
			return groupedOperations{
				tag:        group.Key,
				operations: operations,
			}
		}).
		ToSlice(&result)

	return result
}

// handler generates handler interface for a service
func (generator *Generator) handler(name string, serviceName string, routerName string, hasSchemas bool, operations []operationWithPath) jen.Code {
	var methods []jen.Code

	linq.From(operations).
		ForEachT(func(operation operationWithPath) {
			methodName := generator.normalizer.normalizeOperationName(operation.path, cast.ToString(operation.method))

			if operation.operation.RequestBody == nil || (operation.operation.RequestBody.Value != nil && len(operation.operation.RequestBody.Value.Content) <= 1) {
				requestName := methodName + SuffixRequest
				responseName := methodName + SuffixResponse
				methods = append(methods, jen.Id(methodName).Params(
					jen.Id("ctx").Qual("context", "Context"), jen.Id("request").Op("*").Qual(generator.config.Package, requestName),
				).Params(
					jen.Id("response").Op("*").Qual(generator.config.Package, responseName),
				))
				return
			}

			// Handle multiple content types
			linq.From(operation.operation.RequestBody.Value.Content).
				ForEachT(func(kv linq.KeyValue) {
					contentType := cast.ToString(kv.Key)
					contentMethodName := methodName + generator.normalizer.contentType(contentType)
					requestName := contentMethodName + SuffixRequest
					responseName := methodName + SuffixResponse

					methods = append(methods, jen.Id(contentMethodName).Params(
						jen.Id("ctx").Qual("context", "Context"), jen.Id("request").Op("*").Qual(generator.config.Package, requestName),
					).Params(
						jen.Id("response").Op("*").Qual(generator.config.Package, responseName),
					))
				})
		})

	return jen.Type().Id(serviceName).Interface(methods...)
}

// router generates router struct and constructor
func (generator *Generator) router(routerName string, serviceName string, hasSecuritySchemas bool) jen.Code {
	structFields := []jen.Code{
		jen.Id("service").Id(serviceName),
		jen.Id("router").Op("*").Qual("github.com/go-chi/chi/v5", "Mux"),
		jen.Id("hooks").Op("*").Id("Hooks"),
	}

	if hasSecuritySchemas {
		structFields = append(structFields, jen.Id("processors").Index().Id("securityProcessor"))
	}

	constructorParams := []jen.Code{
		jen.Id("service").Id(serviceName),
	}

	if hasSecuritySchemas {
		constructorParams = append(constructorParams, jen.Id("processors").Op("...").Id("securityProcessor"))
	}

	constructorBody := []jen.Code{
		jen.Id("router").Op(":=").Qual("github.com/go-chi/chi/v5", "NewMux").Call(),
		jen.Id("instance").Op(":=").Op("&").Id(routerName).Values(
			jen.Id("service").Op(":").Id("service"),
			jen.Id("router").Op(":").Id("router"),
			jen.Id("hooks").Op(":").Op("&").Id("Hooks").Values(),
		),
	}

	if hasSecuritySchemas {
		constructorBody[1] = jen.Id("instance").Op(":=").Op("&").Id(routerName).Values(
			jen.Id("service").Op(":").Id("service"),
			jen.Id("router").Op(":").Id("router"),
			jen.Id("hooks").Op(":").Op("&").Id("Hooks").Values(),
			jen.Id("processors").Op(":").Id("processors"),
		)
	}

	constructorBody = append(constructorBody, jen.Return().Id("instance"))

	return jen.Add(
		jen.Type().Id(routerName).Struct(structFields...),
		jen.Line(), jen.Line(),
		jen.Func().Id("New"+routerName).Params(constructorParams...).Op("*").Id(routerName).Block(constructorBody...),
	)
}

// hooksStruct generates the Hooks struct for lifecycle callbacks
func (generator *Generator) hooksStruct() jen.Code {
	return jen.Type().Id("Hooks").Struct(
		jen.Id("RequestSecurityParseFailed").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestSecurityParseCompleted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string")),
		jen.Id("RequestSecurityCheckFailed").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestSecurityCheckCompleted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("string")),
		jen.Id("RequestBodyUnmarshalFailed").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestHeaderParseFailed").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestPathParseFailed").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestQueryParseFailed").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestBodyValidationFailed").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestHeaderValidationFailed").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestPathValidationFailed").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestQueryValidationFailed").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestBodyUnmarshalCompleted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string")),
		jen.Id("RequestHeaderParseCompleted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string")),
		jen.Id("RequestPathParseCompleted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string")),
		jen.Id("RequestQueryParseCompleted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string")),
		jen.Id("RequestParseCompleted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string")),
		jen.Id("RequestProcessingCompleted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string")),
		jen.Id("RequestRedirectStarted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"), jen.Id("string")),
		jen.Id("ResponseBodyMarshalCompleted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string")),
		jen.Id("ResponseBodyWriteCompleted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"), jen.Id("int")),
		jen.Id("ResponseBodyMarshalFailed").Func().Params(
			jen.Qual(PackageNetHTTP, "ResponseWriter"),
			jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("error")),
		jen.Id("ResponseBodyWriteFailed").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string"),
			jen.Id("int"),
			jen.Id("error")),
		jen.Id("ServiceCompleted").Func().Params(jen.Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("string")),
	)
}

// requestProcessingResultType generates types for request processing results
func (generator *Generator) requestProcessingResultType() jen.Code {
	return jen.Type().Id("requestProcessingResultType").Id("uint8").
		Add(jen.Line(), jen.Line()).
		Add(jen.Const().Defs(
			jen.Id("BodyUnmarshalFailed").Id("requestProcessingResultType").Op("=").Id("iota").Op("+").Lit(1),
			jen.Id("BodyValidationFailed"),
			jen.Id("HeaderParseFailed"),
			jen.Id("HeaderValidationFailed"),
			jen.Id("QueryParseFailed"),
			jen.Id("QueryValidationFailed"),
			jen.Id("PathParseFailed"),
			jen.Id("PathValidationFailed"),
			jen.Id("SecurityParseFailed"),
			jen.Id("SecurityCheckFailed"),
			jen.Id("ParseSucceed"),
		)).
		Add(jen.Line(), jen.Line()).
		Add(jen.Type().Id("RequestProcessingResult").Struct(
			jen.Id("error").Id("error"),
			jen.Id("typee").Id("requestProcessingResultType"),
		)).
		Add(jen.Line(), jen.Line()).
		Add(jen.Func().Id("NewRequestProcessingResult").Params(
			jen.Id("t").Id("requestProcessingResultType"),
			jen.Id("err").Id("error")).
			Params(jen.Id("RequestProcessingResult")).Block(
			jen.Return().Id("RequestProcessingResult").Values(jen.Dict{
				jen.Id("typee"): jen.Id("t"),
				jen.Id("error"): jen.Id("err"),
			}))).
		Add(jen.Line(), jen.Line()).
		Add(jen.Func().Params(
			jen.Id("r").Id("RequestProcessingResult")).Id("Type").Params().Params(
			jen.Id("requestProcessingResultType")).Block(
			jen.Return().Id("r").Dot("typee"))).
		Add(jen.Line(), jen.Line()).
		Add(jen.Func().Params(
			jen.Id("r").Id("RequestProcessingResult")).Id("Err").Params().Params(
			jen.Id("error")).Block(
			jen.Return().Id("r").Dot("error"),
		))
}

// wrapperRequestParser generates wrapper request parser functions
func (generator *Generator) wrapperRequestParser(name string, requestName string, routerName, method string, path string, operation *openapi3.Operation, requestBody *openapi3.SchemaRef, contentType string) jen.Code {
	funcCode := []jen.Code{
		jen.Id("request").Dot(FieldProcessingResult).Op("=").Id("RequestProcessingResult").Values(jen.Id("typee").Op(":").Id("ParseSucceed")).Line(),
	}

	funcCode = append(funcCode, generator.wrapperSecurity(name, operation))
	funcCode = append(funcCode, generator.wrapperRequestParsers(name, operation)...)
	funcCode = append(funcCode, generator.wrapperBody(method, path, contentType, name, operation, requestBody))
	funcCode = append(funcCode, jen.Line().If(jen.Id("router").Dot("hooks").Dot("RequestParseCompleted").Op("!=").Id("nil")).Block(
		jen.Id("router").Dot("hooks").Dot("RequestParseCompleted").Call(
			jen.Id("r"),
			jen.Lit(name))))
	funcCode = append(funcCode, jen.Line().Return())

	return jen.Func().Params(
		jen.Id("router").Op("*").Id(routerName)).Id("parse" + name + "Request").
		Params(jen.Id("r").Op("*").Qual(PackageNetHTTP, "Request")).
		Params(jen.Id("request").Id(requestName)).
		Block(funcCode...).
		Line()
}