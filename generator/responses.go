package generator

import (
	"github.com/ahmetb/go-linq"
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cast"
)

// operationResponse represents a response configuration for an operation
type operationResponse struct {
	ContentTypeBodyNameMap map[string]string
	Headers                map[string]*openapi3.HeaderRef
	SetCookie              bool
	StatusCode             string
}

// operationStruct represents structured operation data for response generation
type operationStruct struct {
	Tag                   string
	Name                  string
	RequestName           string
	ResponseName          string
	Responses             []operationResponse
	InterfaceResponseName string
	PrivateName           string
}

// requestResponseBuilders generates request/response builders and related types
func (generator *Generator) requestResponseBuilders(swagger *openapi3.T) jen.Code {
	result := []jen.Code{
		generator.responseStruct(),
		generator.handlersTypes(swagger),
		generator.builders(swagger),
		generator.handlersInterfaces(swagger),
		generator.requestParameters(swagger.Paths.Map()),
	}

	result = generator.normalizer.doubleLineAfterEachElement(result...)

	return jen.Null().Add(result...)
}

// builders generates response builder types and methods
func (generator *Generator) builders(swagger *openapi3.T) (result jen.Code) {
	var builderTypes []jen.Code

	linq.From(generator.groupedOperations(swagger)).
		SelectManyT(func(groupedOperations groupedOperations) linq.Query {
			return linq.From(groupedOperations.operations).
				SelectT(func(operation operationWithPath) operationStruct {
					name := generator.normalizer.normalizeOperationName(operation.path, cast.ToString(operation.method))

					var responses []operationResponse
					linq.From(operation.operation.Responses.Map()).
						SelectT(func(kv linq.KeyValue) operationResponse {
							statusCode := cast.ToString(kv.Key)
							response := kv.Value.(*openapi3.ResponseRef)

							contentTypeBodyNameMap := make(map[string]string)
							if response.Value.Content != nil {
								linq.From(response.Value.Content).
									ForEachT(func(kv linq.KeyValue) {
										contentType := cast.ToString(kv.Key)
										contentTypeBodyNameMap[contentType] = generator.bodyGeneratorName(name, contentType)
									})
							}

							return operationResponse{
								ContentTypeBodyNameMap: contentTypeBodyNameMap,
								Headers:                response.Value.Headers,
								SetCookie:              false, // TODO: detect set-cookie headers
								StatusCode:             statusCode,
							}
						}).
						ToSlice(&responses)

					return operationStruct{
						Tag:                   generator.normalizer.normalize(cast.ToString(groupedOperations.tag)),
						Name:                  name,
						RequestName:           name + SuffixRequest,
						ResponseName:          name + SuffixResponse,
						Responses:             responses,
						InterfaceResponseName: name + "ResponseInterface",
						PrivateName:           generator.normalizer.decapitalize(name),
					}
				})
		}).
		SelectT(func(operation operationStruct) jen.Code {
			return generator.responseBuilders(operation)
		}).
		ToSlice(&builderTypes)

	if len(builderTypes) == 0 {
		return jen.Null()
	}

	builderTypes = generator.normalizer.doubleLineAfterEachElement(builderTypes...)
	return jen.Add(builderTypes...)
}

// handlersTypes generates handler type definitions for operations  
func (generator *Generator) handlersTypes(swagger *openapi3.T) jen.Code {
	var handlerTypes []jen.Code

	linq.From(generator.groupedOperations(swagger)).
		SelectT(func(groupedOperations groupedOperations) jen.Code {
			tag := generator.normalizer.normalize(cast.ToString(groupedOperations.tag))
			return generator.responseType(tag)
		}).
		ToSlice(&handlerTypes)

	if len(handlerTypes) == 0 {
		return jen.Null()
	}

	handlerTypes = generator.normalizer.doubleLineAfterEachElement(handlerTypes...)
	return jen.Add(handlerTypes...)
}

// handlersInterfaces generates handler interface definitions
func (generator *Generator) handlersInterfaces(swagger *openapi3.T) jen.Code {
	var interfaceTypes []jen.Code

	linq.From(generator.groupedOperations(swagger)).
		SelectManyT(func(groupedOperations groupedOperations) linq.Query {
			return linq.From(groupedOperations.operations).
				SelectT(func(operation operationWithPath) jen.Code {
					name := generator.normalizer.normalizeOperationName(operation.path, cast.ToString(operation.method))
					return generator.responseInterface(name)
				})
		}).
		ToSlice(&interfaceTypes)

	if len(interfaceTypes) == 0 {
		return jen.Null()
	}

	interfaceTypes = generator.normalizer.doubleLineAfterEachElement(interfaceTypes...)
	return jen.Add(interfaceTypes...)
}

// responseStruct generates the base Response struct
func (generator *Generator) responseStruct() jen.Code {
	return jen.Type().Id("Response").Struct(
		jen.Id("body").Interface(),
		jen.Id("contentType").String(),
		jen.Id("statusCode").Int(),
		jen.Id("headers").Map(jen.String()).String(),
	).Line().Line().
		Func().Params(jen.Id("r").Op("*").Id("Response")).Id("WriteTo").Params(
			jen.Id("w").Qual(PackageNetHTTP, "ResponseWriter")).Block(
		jen.For(jen.List(jen.Id("key"), jen.Id("value")).Op(":=").Range().Id("r").Dot("headers")).Block(
			jen.Id("w").Dot("Header").Call().Dot("Set").Call(jen.Id("key"), jen.Id("value"))),
		jen.Id("w").Dot("Header").Call().Dot("Set").Call(jen.Lit("Content-Type"), jen.Id("r").Dot("contentType")),
		jen.Id("w").Dot("WriteHeader").Call(jen.Id("r").Dot("statusCode")),
		jen.Switch(jen.Id("body").Op(":=").Id("r").Dot("body").Assert(jen.Type())).Block(
			jen.Case(jen.String()).Block(
				jen.Id("w").Dot("Write").Call(jen.Index().Byte().Call(jen.Id("body"))),
			),
			jen.Case(jen.Index().Byte()).Block(
				jen.Id("w").Dot("Write").Call(jen.Id("body")),
			),
			jen.Default().Block(
				jen.If(jen.Id("r").Dot("body").Op("!=").Nil()).Block(
					jen.Qual(PackageEncodingJSON, "NewEncoder").Call(jen.Id("w")).Dot("Encode").Call(jen.Id("r").Dot("body")),
				),
			),
		),
	)
}

// responseInterface generates response interface for operations
func (generator *Generator) responseInterface(name string) jen.Code {
	interfaceName := name + "ResponseInterface"
	return jen.Type().Id(interfaceName).Interface(
		jen.Id("WriteTo").Params(jen.Qual(PackageNetHTTP, "ResponseWriter")),
	)
}

// responseType generates response type definitions
func (generator *Generator) responseType(name string) jen.Code {
	typeName := name + SuffixResponse
	return jen.Type().Id(typeName).Interface(
		jen.Id("WriteTo").Params(jen.Qual(PackageNetHTTP, "ResponseWriter")),
	)
}

// responseBuilders generates response builder methods for an operation
func (generator *Generator) responseBuilders(operationStruct operationStruct) jen.Code {
	var builders []jen.Code

	for _, response := range operationStruct.Responses {
		statusCode := response.StatusCode
		
		// Generate status code builder
		statusBuilderName := operationStruct.Name + "Status" + statusCode + "Builder"
		nextBuilderName := generator.contentTypeBuilderName(operationStruct.Name + statusCode)
		
		statusBuilder := generator.responseStatusCodeBuilder(response, statusBuilderName, nextBuilderName)
		builders = append(builders, statusBuilder...)

		// Generate content type builders
		for contentType, bodyBuilderName := range response.ContentTypeBodyNameMap {
			contentTypeName := generator.contentTypeFuncName(contentType)
			contentTypeBuilderName := generator.contentTypeBuilderName(operationStruct.Name + statusCode + contentTypeName)
			
			headerBuilderName := operationStruct.Name + statusCode + contentTypeName + "HeadersBuilder"
			
			contentTypeBuilder := generator.responseContentTypeBuilder(contentTypeName, contentType, contentTypeBuilderName, bodyBuilderName, headerBuilderName, response.Headers)
			builders = append(builders, contentTypeBuilder...)
		}
	}

	if len(builders) == 0 {
		return jen.Null()
	}

	builders = generator.normalizer.doubleLineAfterEachElement(builders...)
	return jen.Add(builders...)
}

// responseContentTypeBuilder generates content type specific builders
func (generator *Generator) responseContentTypeBuilder(contentTypeName string, contentType string, contentTypeBuilderName string, bodyBuilderName string, nextBuilderName string, headers map[string]*openapi3.HeaderRef) (results []jen.Code) {
	// Generate content type builder struct
	results = append(results, jen.Type().Id(contentTypeBuilderName).Struct(
		jen.Id("response").Op("*").Id("Response"),
	))

	// Generate body builder method
	results = append(results, jen.Func().Params(jen.Id("b").Op("*").Id(contentTypeBuilderName)).Id(bodyBuilderName).Params(
		jen.Id("body").Interface()).Op("*").Id(nextBuilderName).Block(
		jen.Id("b").Dot("response").Dot("body").Op("=").Id("body"),
		jen.Id("b").Dot("response").Dot("contentType").Op("=").Lit(contentType),
		jen.Return().Op("&").Id(nextBuilderName).Values(jen.Id("response").Op(":").Id("b").Dot("response")),
	))

	return results
}

// responseStatusCodeBuilder generates status code builders
func (generator *Generator) responseStatusCodeBuilder(resp operationResponse, builderName string, nextBuilderName string) (results []jen.Code) {
	statusCode := cast.ToInt(resp.StatusCode)

	// Generate status code builder struct
	results = append(results, jen.Type().Id(builderName).Struct(
		jen.Id("response").Op("*").Id("Response"),
	))

	// Generate status code setter method
	results = append(results, jen.Func().Params(jen.Id("b").Op("*").Id(builderName)).Id("Status").Params().Op("*").Id(nextBuilderName).Block(
		jen.Id("b").Dot("response").Dot("statusCode").Op("=").Lit(statusCode),
		jen.Return().Op("&").Id(nextBuilderName).Values(jen.Id("response").Op(":").Id("b").Dot("response")),
	))

	return results
}

// responseHeadersBuilder generates header builders
func (generator *Generator) responseHeadersBuilder(headers map[string]*openapi3.HeaderRef, headersStructName string, headersBuilderName string, nextBuilderName string) (results []jen.Code) {
	if len(headers) == 0 {
		return nil
	}

	// Generate headers struct
	var headerFields []jen.Code
	for headerName, headerRef := range headers {
		fieldName := generator.normalizer.normalize(headerName)
		field := jen.Id(fieldName)
		
		generator.typee.fillGoType(field, "", fieldName, headerRef.Value.Schema, false, false)
		generator.typee.fillJsonTag(field, headerRef.Value.Schema, headerName)
		
		headerFields = append(headerFields, field)
	}

	results = append(results, jen.Type().Id(headersStructName).Struct(headerFields...))

	// Generate headers builder method
	results = append(results, jen.Func().Params(jen.Id("b").Op("*").Id(headersBuilderName)).Id("Headers").Params(
		jen.Id("headers").Id(headersStructName)).Op("*").Id(nextBuilderName).Block(
		jen.Return().Op("&").Id(nextBuilderName).Values(jen.Id("response").Op(":").Id("b").Dot("response")),
	))

	return results
}

// responseCookiesBuilder generates cookie builders
func (generator *Generator) responseCookiesBuilder(cookieBuilderName string, nextBuilderName string) (results []jen.Code) {
	results = append(results, jen.Func().Params(jen.Id("b").Op("*").Id(cookieBuilderName)).Id("SetCookie").Params(
		jen.Id("cookie").Op("*").Qual(PackageNetHTTP, "Cookie")).Op("*").Id(nextBuilderName).Block(
		jen.Comment("// TODO: Implement cookie setting"),
		jen.Return().Op("&").Id(nextBuilderName).Values(jen.Id("response").Op(":").Id("b").Dot("response")),
	))

	return results
}

// responseAssembler generates response assembler
func (generator *Generator) responseAssembler(assemblerName string, interfaceResponseName string, responseName string) (results []jen.Code) {
	results = append(results, jen.Func().Params(jen.Id("b").Op("*").Id(assemblerName)).Id("Build").Params().Id(interfaceResponseName).Block(
		jen.Return().Id("b").Dot("response"),
	))

	return results
}

// headersStruct generates header struct definitions
func (generator *Generator) headersStruct(name string, headers map[string]*openapi3.HeaderRef) jen.Code {
	if len(headers) == 0 {
		return jen.Null()
	}

	var fields []jen.Code
	for headerName, headerRef := range headers {
		fieldName := generator.normalizer.normalize(headerName)
		field := jen.Id(fieldName)

		if headerRef.Value != nil && headerRef.Value.Schema != nil {
			generator.typee.fillGoType(field, name, fieldName, headerRef.Value.Schema, false, false)
			generator.typee.fillJsonTag(field, headerRef.Value.Schema, headerName)
		} else {
			field.String().Tag(map[string]string{"json": headerName})
		}

		fields = append(fields, field)
	}

	headersStructName := name + "Headers"
	return jen.Type().Id(headersStructName).Struct(fields...)
}

// responseImplementationFunc generates response implementation functions
func (generator *Generator) responseImplementationFunc(name string) jen.Code {
	implFuncName := name + "ResponseImpl"
	interfaceName := name + "ResponseInterface"
	
	return jen.Func().Id(implFuncName).Params().Id(interfaceName).Block(
		jen.Return().Op("&").Id("Response").Values(),
	)
}