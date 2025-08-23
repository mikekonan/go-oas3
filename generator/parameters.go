package generator

import (
	"fmt"

	"github.com/ahmetb/go-linq"
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cast"
)

// requestParameters generates request parameter structs for all paths
func (generator *Generator) requestParameters(paths map[string]*openapi3.PathItem) jen.Code {
	var result []jen.Code

	linq.From(paths).
		SelectManyT(func(kv linq.KeyValue) linq.Query {
			path := cast.ToString(kv.Key)
			operationsCodeTags := map[string][]jen.Code{}

			linq.From(kv.Value.(*openapi3.PathItem).Operations()).
				GroupByT(
					func(kv linq.KeyValue) string {
						operation := kv.Value.(*openapi3.Operation)
						if len(operation.Tags) == 0 {
							return ParamDefault
						}
						return generator.normalizer.normalize(operation.Tags[0])
					},
					func(kv linq.KeyValue) (result []jen.Code) {
						name := generator.normalizer.normalizeOperationName(path, cast.ToString(kv.Key))
						operation := kv.Value.(*openapi3.Operation)
						
						// Always generate a request struct, even if empty
						if operation.RequestBody == nil {
							result = append(result, generator.requestParameterStruct(name, "", false, operation))
							return
						}

						if operation.RequestBody != nil && operation.RequestBody.Value != nil && len(operation.RequestBody.Value.Content) == 1 {
							contentType := cast.ToString(linq.From(operation.RequestBody.Value.Content).SelectT(func(kv linq.KeyValue) string { return cast.ToString(kv.Key) }).First())
							result = append(result, generator.requestParameterStruct(name, contentType, false, operation))
							return
						}

						var contentTypeResult []jen.Code
						if operation.RequestBody != nil && operation.RequestBody.Value != nil {
							linq.From(operation.RequestBody.Value.Content).
							SelectT(func(kv linq.KeyValue) jen.Code {
								return generator.requestParameterStruct(name, cast.ToString(kv.Key), true, operation)
							}).
							ToSlice(&contentTypeResult)
						}

						result = append(result, contentTypeResult...)
						result = generator.normalizer.doubleLineAfterEachElement(result...)

						return
					}).
				ForEachT(func(group linq.Group) {
					if _, ok := operationsCodeTags[cast.ToString(group.Key)]; !ok {
						operationsCodeTags[cast.ToString(group.Key)] = []jen.Code{}
					}

					// Extract the grouped jen.Code values
					linq.From(group.Group).ForEachT(func(item interface{}) {
						if codes, ok := item.([]jen.Code); ok {
							operationsCodeTags[cast.ToString(group.Key)] = append(operationsCodeTags[cast.ToString(group.Key)], codes...)
						}
					})
				})

			return linq.From(operationsCodeTags).SelectT(func(kv linq.KeyValue) jen.Code {
				return jen.Add(generator.normalizer.doubleLineAfterEachElement(kv.Value.([]jen.Code)...)...)
			})
		}).
		ToSlice(&result)

	if len(result) == 0 {
		return jen.Null()
	}

	result = generator.normalizer.doubleLineAfterEachElement(result...)
	return jen.Add(result...)
}

// requestParameterStruct generates request parameter struct for a specific operation
func (generator *Generator) requestParameterStruct(name string, contentType string, appendContentTypeToName bool, operation *openapi3.Operation) jen.Code {
	type parameter struct {
		In   string
		Code jen.Code
	}

	var additionalParameters []parameter

	// Handle request body
	if contentType != "" && operation.RequestBody != nil && operation.RequestBody.Value != nil {
		if appendContentTypeToName {
			name += generator.normalizer.contentType(contentType)
		}

		bodyTypeName := generator.normalizer.extractNameFromRef(operation.RequestBody.Value.Content[contentType].Schema.Ref)
		if bodyTypeName == "" {
			bodyTypeName = name + SuffixRequestBody
		}

		additionalParameters = append(additionalParameters,
			parameter{In: InBody, Code: jen.Id(InBody).Qual(generator.config.ComponentsPackage, bodyTypeName)})
	}

	var parameterStructs []jen.Code

	// Group parameters by location (in: header, path, query)
	linq.From(operation.Parameters).
		GroupByT(
			func(parameter *openapi3.ParameterRef) string { return parameter.Value.In },
			func(parameter *openapi3.ParameterRef) *openapi3.ParameterRef { return parameter }).
		SelectT(
			func(group linq.Group) jen.Code {
				in := cast.ToString(group.Key)
				parameters := group.Group

				var structProperties []jen.Code

				linq.From(parameters).
					SelectT(func(parameter *openapi3.ParameterRef) jen.Code {
						propertyName := generator.normalizer.normalize(parameter.Value.Name)
						field := jen.Id(propertyName)

						generator.typee.fillGoType(field, name, propertyName, parameter.Value.Schema, !parameter.Value.Required, false)
						generator.typee.fillJsonTag(field, parameter.Value.Schema, parameter.Value.Name)

						return field
					}).
					ToSlice(&structProperties)

				if len(structProperties) == 0 {
					return jen.Null()
				}

				return jen.Id(in).Struct(structProperties...)
			}).
		WhereT(func(code jen.Code) bool { return code != jen.Null() }).
		ToSlice(&parameterStructs)

	// Add additional parameters (like request body)
	for _, param := range additionalParameters {
		parameterStructs = append(parameterStructs, param.Code)
	}

	requestName := name + SuffixRequest
	
	// If no parameters, create empty struct with ProcessingResult
	if len(parameterStructs) == 0 {
		return jen.Type().Id(requestName).Struct(
			jen.Id(FieldProcessingResult).Qual(generator.config.ComponentsPackage, RequestProcessingResult),
		)
	}

	// Add ProcessingResult to all request structs
	parameterStructs = append(parameterStructs, jen.Id(FieldProcessingResult).Qual(generator.config.ComponentsPackage, RequestProcessingResult))
	
	return jen.Type().Id(requestName).Struct(parameterStructs...)
}

// wrapperRequestParsers generates request parsers for wrapper functions
func (generator *Generator) wrapperRequestParsers(wrapperName string, operation *openapi3.Operation) (result []jen.Code) {
	var parsers []jen.Code

	// Generate parsers for each parameter location
	linq.From(operation.Parameters).
		GroupByT(
			func(parameter *openapi3.ParameterRef) string { return parameter.Value.In },
			func(parameter *openapi3.ParameterRef) *openapi3.ParameterRef { return parameter }).
		SelectT(func(group linq.Group) jen.Code {
			in := cast.ToString(group.Key)
			parameters := group.Group

			var parserStatements []jen.Code

			linq.From(parameters).
				SelectT(func(parameter *openapi3.ParameterRef) jen.Code {
					return generator.generateParameterParser(in, parameter, wrapperName)
				}).
				WhereT(func(code jen.Code) bool { return code != jen.Null() }).
				ToSlice(&parserStatements)

			if len(parserStatements) == 0 {
				return jen.Null()
			}

			return jen.Add(parserStatements...)
		}).
		WhereT(func(code jen.Code) bool { return code != jen.Null() }).
		ToSlice(&parsers)

	return parsers
}

// generateParameterParser generates parser code for a specific parameter
func (generator *Generator) generateParameterParser(in string, parameter *openapi3.ParameterRef, wrapperName string) jen.Code {
	param := parameter.Value
	propertyName := generator.normalizer.normalize(param.Name)
	paramName := generator.normalizer.normalize(param.Name)

	switch in {
	case InPath:
		return generator.generatePathParameterParser(propertyName, paramName, wrapperName, parameter)
	case InQuery:
		return generator.generateQueryParameterParser(propertyName, paramName, wrapperName, parameter)
	case InHeader:
		return generator.generateHeaderParameterParser(propertyName, paramName, wrapperName, parameter)
	default:
		return jen.Null()
	}
}

// generatePathParameterParser generates parser for path parameters
func (generator *Generator) generatePathParameterParser(propertyName, paramName, wrapperName string, parameter *openapi3.ParameterRef) jen.Code {
	param := parameter.Value

	if param.Schema != nil && param.Schema.Value != nil && param.Schema.Value.Type.Is(TypeString) {
		if generator.typee.isCustomType(param.Schema.Value) {
			return generator.wrapperCustomType(InPath, propertyName, paramName, wrapperName, parameter)
		}

		if len(param.Schema.Value.Enum) > 0 {
			enumType := generator.normalizer.extractNameFromRef(param.Schema.Ref)
			if enumType == "" {
				enumType = generator.normalizer.normalize(propertyName) + SuffixEnum
			}
			return generator.wrapperEnum(InPath, enumType, propertyName, paramName, wrapperName, parameter)
		}

		return generator.wrapperStr(InPath, propertyName, paramName, wrapperName, parameter)
	}

	if param.Schema != nil && param.Schema.Value != nil && param.Schema.Value.Type.Is(TypeInteger) {
		return generator.wrapperInteger(InPath, propertyName, paramName, wrapperName, parameter)
	}

	return jen.Null()
}

// generateQueryParameterParser generates parser for query parameters
func (generator *Generator) generateQueryParameterParser(propertyName, paramName, wrapperName string, parameter *openapi3.ParameterRef) jen.Code {
	param := parameter.Value

	if param.Schema != nil && param.Schema.Value != nil && param.Schema.Value.Type.Is(TypeString) {
		if generator.typee.isCustomType(param.Schema.Value) {
			return generator.wrapperCustomType(InQuery, propertyName, paramName, wrapperName, parameter)
		}

		if len(param.Schema.Value.Enum) > 0 {
			enumType := generator.normalizer.extractNameFromRef(param.Schema.Ref)
			if enumType == "" {
				enumType = generator.normalizer.normalize(propertyName) + SuffixEnum
			}
			return generator.wrapperEnum(InQuery, enumType, propertyName, paramName, wrapperName, parameter)
		}

		return generator.wrapperStr(InQuery, propertyName, paramName, wrapperName, parameter)
	}

	if param.Schema != nil && param.Schema.Value != nil && param.Schema.Value.Type.Is(TypeInteger) {
		return generator.wrapperInteger(InQuery, propertyName, paramName, wrapperName, parameter)
	}

	return jen.Null()
}

// generateHeaderParameterParser generates parser for header parameters
func (generator *Generator) generateHeaderParameterParser(propertyName, paramName, wrapperName string, parameter *openapi3.ParameterRef) jen.Code {
	param := parameter.Value

	if param.Schema != nil && param.Schema.Value != nil && param.Schema.Value.Type.Is(TypeString) {
		if generator.typee.isCustomType(param.Schema.Value) {
			return generator.wrapperCustomType(InHeader, propertyName, paramName, wrapperName, parameter)
		}

		if len(param.Schema.Value.Enum) > 0 {
			enumType := generator.normalizer.extractNameFromRef(param.Schema.Ref)
			if enumType == "" {
				enumType = generator.normalizer.normalize(propertyName) + SuffixEnum
			}
			return generator.wrapperEnum(InHeader, enumType, propertyName, paramName, wrapperName, parameter)
		}

		return generator.wrapperStr(InHeader, propertyName, paramName, wrapperName, parameter)
	}

	if param.Schema != nil && param.Schema.Value != nil && param.Schema.Value.Type.Is(TypeInteger) {
		return generator.wrapperInteger(InHeader, propertyName, paramName, wrapperName, parameter)
	}

	return jen.Null()
}

// wrapperCustomType generates wrapper code for custom type parameters
func (generator *Generator) wrapperCustomType(in string, name string, paramName string, wrapperName string, parameter *openapi3.ParameterRef) jen.Code {
	result := jen.Null()

	switch in {
	case InHeader:
		result = result.Add(jen.Id(paramName + ParamStr).Op(":=").Id(VarR).Dot(HTTPHeader).Dot(HTTPGet).Call(jen.Lit(parameter.Value.Name)))
	case InQuery:
		result = result.Add(jen.Id(paramName + ParamStr).Op(":=").Id(VarR).Dot(HTTPURL).Dot(HTTPQuery).Call().Dot(HTTPGet).Call(jen.Lit(parameter.Value.Name)))
	case InPath:
		result = result.Add(jen.Id(paramName+ParamStr).Op(":=").Id(VarChi).Dot(URLParam).Call(jen.Id(VarR), jen.Lit(parameter.Value.Name)))
	default:
		PanicInvalidOperation("Parameter Parsing", "unsupported parameter location", map[string]interface{}{"parameter_in": in, "supported_types": "header, path, query"})
	}

	result = result.Add(jen.Line())

	parseFailed := []jen.Code{
		jen.Id(VarRequest).Dot(FieldProcessingResult).Op("=").Qual(generator.config.ComponentsPackage, RequestProcessingResult).Values(jen.Id(ParamError).Op(":").Id(VarErr),
			jen.Id(ParamType).Op(":").Id(generator.normalizer.titleCase(in)+"ParseFailed")),
		jen.If(jen.Id(VarRouter).Dot(VarHooks).Dot("Request" + generator.normalizer.titleCase(in) + "ParseFailed").Op("!=").Id(ParamNil)).Block(
			jen.Id(VarRouter).Dot(VarHooks).Dot("Request"+generator.normalizer.titleCase(in)+"ParseFailed").Call(
				jen.Id(VarR),
				jen.Lit(wrapperName),
				jen.Lit(parameter.Value.Name),
				jen.Id(VarRequest).Dot(FieldProcessingResult))),
		jen.Line().Return(),
	}

	if pkg, parse, ok := generator.typee.getXGoTypeStringParse(parameter.Value.Schema.Value); ok {
		parameterCode := jen.Null().
			Add(jen.List(jen.Id(paramName), jen.Id(VarErr)).Op(":=").Qual(pkg, parse).Call(jen.Id(paramName+"Str"))).
			Add(jen.Line()).
			Add(jen.If(jen.Id(VarErr).Op("!=").Id(ParamNil)).Block(parseFailed...)).
			Add(jen.Line(), jen.Line()).
			Add(func() jen.Code {
				if parameter.Value.Required {
					return jen.Id(VarRequest).Dot(in).Dot(name).Op("=").Id(paramName)
				} else {
					return jen.Id(VarRequest).Dot(in).Dot(name).Op("=").Op("&").Id(paramName)
				}
			}())

		result.Add(generator.wrapRequired(paramName+"Str", parameter.Value.Required, parameterCode))
	} else {
		ref := generator.extractRefFromAllOf(parameter.Value.Schema)
		if ref != "" {
			parameter.Value.Schema.Ref = ref
		}

		if parameter.Value.Schema != nil && parameter.Value.Schema.Value != nil {
			switch parameter.Value.Schema.Value.Format {
		case FormatUUID:
			parameterCode := jen.Null().
				Add(jen.List(jen.Id(paramName), jen.Id(VarErr)).Op(":=").Id("uuid").Dot("Parse").Call(jen.Id(paramName+"Str"))).
				Add(jen.Line()).
				Add(jen.If(jen.Id(VarErr).Op("!=").Id(ParamNil)).Block(parseFailed...)).
				Add(jen.Line(), jen.Line()).
				Add(func() jen.Code {
					if parameter.Value.Required {
						return jen.Id(VarRequest).Dot(in).Dot(name).Op("=").Id(paramName)
					} else {
						return jen.Id(VarRequest).Dot(in).Dot(name).Op("=").Op("&").Id(paramName)
					}
				}())

			result.Add(generator.wrapRequired(paramName+"Str", parameter.Value.Required, parameterCode))
		case FormatISO4217CurrencyCode:
			parameterCode := jen.Null().
				Add(jen.List(jen.Id(paramName), jen.Id(VarErr)).Op(":=").Qual("github.com/mikekonan/go-types/v2/currency", "ByCodeStrErr").Call(jen.Id(paramName+"Str"))).
				Add(jen.Line()).
				Add(jen.If(jen.Id(VarErr).Op("!=").Id(ParamNil)).Block(parseFailed...)).
				Add(jen.Line(), jen.Line()).
				Add(jen.Id(VarRequest).Dot(in).Dot(name).Op("=").Id(paramName).Dot("Code").Call())

			result.Add(generator.wrapRequired(paramName+"Str", parameter.Value.Required, parameterCode))
		case FormatISO3166Alpha2:
			parameterCode := jen.Null().
				Add(jen.List(jen.Id(paramName), jen.Id(VarErr)).Op(":=").Qual("github.com/mikekonan/go-types/v2/country", "ByAlpha2CodeStrErr").Call(jen.Id(paramName+"Str"))).
				Add(jen.Line()).
				Add(jen.If(jen.Id(VarErr).Op("!=").Id(ParamNil)).Block(parseFailed...)).
				Add(jen.Line(), jen.Line()).
				Add(jen.Id(VarRequest).Dot(in).Dot(name).Op("=").Id(paramName).Dot("Alpha2Code").Call())

			result.Add(generator.wrapRequired(paramName+"Str", parameter.Value.Required, parameterCode))
		}
		}
	}

	return result.Line()
}

// wrapperEnum generates wrapper code for enum parameters
func (generator *Generator) wrapperEnum(in string, enumType string, name string, paramName string, wrapperName string, parameter *openapi3.ParameterRef) jen.Code {
	result := jen.Null()

	switch in {
	case InHeader:
		result = result.Add(jen.Id(paramName).Op(":=").Qual(generator.config.ComponentsPackage, enumType).Call(jen.Id(VarR).Dot("Header").Dot("Get").Call(jen.Lit(parameter.Value.Name))))
	case InQuery:
		result = result.Add(jen.Id(paramName).Op(":=").Qual(generator.config.ComponentsPackage, enumType).Call(jen.Id(VarR).Dot("URL").Dot("Query").Call().Dot("Get").Call(jen.Lit(parameter.Value.Name))))
	case InPath:
		result = result.Add(jen.Id(paramName).Op(":=").Qual(generator.config.ComponentsPackage, enumType).Call(jen.Id("chi").Dot("URLParam").Call(jen.Id(VarR), jen.Lit(parameter.Value.Name))))
	default:
		PanicInvalidOperation("Parameter Parsing", "unsupported parameter location", map[string]interface{}{"parameter_in": in, "supported_types": "header, path, query"})
	}

	result = result.
		Add(jen.Line()).
		Add(jen.If(jen.Id(VarErr).Op(":=").Id(paramName).Dot(MethodCheck).Call(),
			jen.Id(VarErr).Op("!=").Id(ParamNil)).Block(
			jen.Id(VarRequest).Dot(FieldProcessingResult).Op("=").Qual(generator.config.ComponentsPackage, RequestProcessingResult).Values(jen.Id(ParamError).Op(":").Id(VarErr),
				jen.Id(ParamType).Op(":").Id(generator.normalizer.titleCase(in)+"ParseFailed")),
			jen.If(jen.Id(VarRouter).Dot(VarHooks).Dot("Request"+generator.normalizer.titleCase(in)+"ParseFailed").Op("!=").Id(ParamNil)).Block(
				jen.Id(VarRouter).Dot(VarHooks).Dot("Request"+generator.normalizer.titleCase(in)+"ParseFailed").Call(
					jen.Id(VarR),
					jen.Lit(wrapperName),
					jen.Lit(parameter.Value.Name),
					jen.Id(VarRequest).Dot(FieldProcessingResult))),
			jen.Line().Return())).
		Add(jen.Line()).
		Add(jen.Id(VarRequest).Dot(in).Dot(name).Op("=").Id(paramName)).
		Add(jen.Line())

	return result
}

// wrapperStr generates wrapper code for string parameters
func (generator *Generator) wrapperStr(in string, name string, paramName string, wrapperName string, parameter *openapi3.ParameterRef) jen.Code {
	result := jen.Null()

	switch in {
	case InHeader:
		result = result.Add(jen.Id(paramName).Op(":=").Id(VarR).Dot("Header").Dot("Get").Call(jen.Lit(parameter.Value.Name)))
	case InQuery:
		result = result.Add(jen.Id(paramName).Op(":=").Id(VarR).Dot("URL").Dot("Query").Call().Dot("Get").Call(jen.Lit(parameter.Value.Name)))
	case InPath:
		result = result.Add(jen.Id(paramName).Op(":=").Id("chi").Dot("URLParam").Call(jen.Id(VarR), jen.Lit(parameter.Value.Name)))
	default:
		PanicInvalidOperation("Parameter Parsing", "unsupported parameter location", map[string]interface{}{"parameter_in": in, "supported_types": "header, path, query"})
	}

	if parameter.Value.Required {
		result = result.
			Add(jen.Line()).
			Add(jen.If(jen.Id(paramName).Op("==").Lit("")).Block(
				jen.Id(VarErr).Op(":=").Qual(PackageFmt, MethodErrorf).Call(jen.Lit(fmt.Sprintf(ErrorFieldRequired, parameter.Value.Name))).Line(),
				jen.Id(VarRequest).Dot(FieldProcessingResult).Op("=").Qual(generator.config.ComponentsPackage, RequestProcessingResult).Values(jen.Id(ParamError).Op(":").Id(VarErr),
					jen.Id(ParamType).Op(":").Id(generator.normalizer.titleCase(in)+"ParseFailed")),
				jen.If(jen.Id(VarRouter).Dot(VarHooks).Dot("Request"+generator.normalizer.titleCase(in)+"ParseFailed").Op("!=").Id(ParamNil)).Block(
					jen.Id(VarRouter).Dot(VarHooks).Dot("Request"+generator.normalizer.titleCase(in)+"ParseFailed").Call(
						jen.Id(VarR),
						jen.Lit(wrapperName),
						jen.Lit(parameter.Value.Name),
						jen.Id(VarRequest).Dot(FieldProcessingResult))),
				jen.Line().Return())).
			Add(jen.Line())
	}

	// Handle regex validation
	regex := generator.getXGoRegex(parameter.Value.Schema)
	if regex != "" {
		// Ensure regex variable is generated and get its name
		regexVarName := generator.useRegex[regex]
		if regexVarName == "" {
			// Generate the regex variable if it doesn't exist
			parameterName := generator.normalizer.normalize(parameter.Value.Name)
			generator.variableForRegex(parameterName, parameter.Value.Schema)
			regexVarName = generator.useRegex[regex]
		}

		result = result.Line().If(jen.Op("!").Id(regexVarName).Dot(MethodMatchString).Call(jen.Id(paramName))).Block(
			jen.Id(VarErr).Op(":=").Qual(PackageFmt, MethodErrorf).Call(jen.Lit(fmt.Sprintf(ErrorRegexNotMatched, parameter.Value.Name, regex))),
			jen.Line(),
			jen.Id(VarRequest).Dot(FieldProcessingResult).Op("=").Qual(generator.config.ComponentsPackage, RequestProcessingResult).Values(jen.Id(ParamError).Op(":").Id(VarErr),
				jen.Id(ParamType).Op(":").Id(fmt.Sprintf("%sParseFailed", generator.normalizer.titleCase(in)))),
			jen.If(jen.Id(VarRouter).Dot(VarHooks).Dot("Request"+generator.normalizer.titleCase(in)+"ParseFailed").Op("!=").Id(ParamNil)).Block(
				jen.Id(VarRouter).Dot(VarHooks).Dot("Request"+generator.normalizer.titleCase(in)+"ParseFailed").Call(jen.Id(VarR),
					jen.Lit(wrapperName),
					jen.Lit(parameter.Value.Name),
					jen.Id(VarRequest).Dot(FieldProcessingResult))),
			jen.Line(),
			jen.Return()).
			Line()
	}

	// For optional parameters, assign address of value to pointer field
	if parameter.Value.Required {
		result = result.
			Line().
			Add(jen.Id(VarRequest).Dot(parameter.Value.In).Dot(name).Op("=").Id(paramName)).
			Line()
	} else {
		// Optional parameter - assign pointer to string
		result = result.
			Line().
			Add(jen.If(jen.Id(paramName).Op("!=").Lit("")).Block(
				jen.Id(VarRequest).Dot(parameter.Value.In).Dot(name).Op("=").Op("&").Id(paramName))).
			Line()
	}

	return result
}

// wrapperInteger generates wrapper code for integer parameters
func (generator *Generator) wrapperInteger(in string, name string, paramName string, wrapperName string, parameter *openapi3.ParameterRef) jen.Code {
	result := jen.Null()

	switch in {
	case InHeader:
		result = result.Add(jen.Id(paramName).Op(":=").Id(VarR).Dot("Header").Dot("Get").Call(jen.Lit(parameter.Value.Name)))
	case InQuery:
		result = result.Add(jen.Id(paramName).Op(":=").Id(VarR).Dot("URL").Dot("Query").Call().Dot("Get").Call(jen.Lit(parameter.Value.Name)))
	case InPath:
		result = result.Add(jen.Id(paramName).Op(":=").Id("chi").Dot("URLParam").Call(jen.Id(VarR), jen.Lit(parameter.Value.Name)))
	default:
		PanicInvalidOperation("Parameter Parsing", "unsupported parameter location", map[string]interface{}{"parameter_in": in, "supported_types": "header, path, query"})
	}

	if parameter.Value.Required {
		result = result.
			Add(jen.Line()).
			Add(jen.If(jen.Id(paramName).Op("==").Lit("")).Block(
				jen.Id(VarErr).Op(":=").Qual(PackageFmt, MethodErrorf).Call(jen.Lit(fmt.Sprintf(ErrorFieldRequired, parameter.Value.Name))).Line(),
				jen.Id(VarRequest).Dot(FieldProcessingResult).Op("=").Qual(generator.config.ComponentsPackage, RequestProcessingResult).Values(jen.Id(ParamError).Op(":").Id(VarErr),
					jen.Id(ParamType).Op(":").Id(generator.normalizer.titleCase(in)+"ParseFailed")),
				jen.If(jen.Id(VarRouter).Dot(VarHooks).Dot("Request"+generator.normalizer.titleCase(in)+"ParseFailed").Op("!=").Id(ParamNil)).Block(
					jen.Id(VarRouter).Dot(VarHooks).Dot("Request"+generator.normalizer.titleCase(in)+"ParseFailed").Call(
						jen.Id(VarR),
						jen.Lit(wrapperName),
						jen.Lit(parameter.Value.Name),
						jen.Id(VarRequest).Dot(FieldProcessingResult))),
				jen.Line().Return())).
			Add(jen.Line())
	}

	// For optional parameters, assign address of value to pointer field
	if parameter.Value.Required {
		result = result.
			Add(jen.Line()).
			Add(jen.Id(VarRequest).Dot(parameter.Value.In).Dot(name).Op("=").Qual("github.com/spf13/cast", "ToInt").Call(jen.Id(paramName))).
			Add(jen.Line())
	} else {
		// Optional parameter - assign pointer to value
		result = result.
			Add(jen.Line()).
			Add(jen.If(jen.Id(paramName).Op("!=").Lit("")).Block(
				jen.List(jen.Id(ParamIntVal), jen.Id(VarErr)).Op(":=").Qual("github.com/spf13/cast", "ToIntE").Call(jen.Id(paramName)),
				jen.If(jen.Id(VarErr).Op("==").Id(ParamNil)).Block(
					jen.Id(VarRequest).Dot(parameter.Value.In).Dot(name).Op("=").Op("&").Id(ParamIntVal)))).
			Add(jen.Line())
	}

	return result
}

// wrapperBody generates wrapper code for request body parsing
func (generator *Generator) wrapperBody(method string, path string, contentType string, wrapperName string, operation *openapi3.Operation, body *openapi3.SchemaRef) jen.Code {
	result := jen.Null()

	if operation.RequestBody == nil {
		return result
	}

	name := generator.normalizer.extractNameFromRef(body.Ref)

	if name == "" {
		name = generator.normalizer.normalizeOperationName(path, method) + generator.normalizer.contentType(cast.ToString(contentType)) + SuffixRequestBody
	}

	result = result.
		Add(jen.Var().Defs(
			jen.Id(VarBody).Qual(generator.config.ComponentsPackage, name),
			jen.Id(ParamDecodeErr).Error(),
		)).
		Add(jen.Line()).
		Add(func() *jen.Statement {
			switch contentType {
			case "application/xml":
				return jen.Id(ParamDecodeErr).Op("=").Qual("encoding/xml", "NewDecoder").Call(jen.Id(VarR).Dot("Body")).Dot("Decode").Call(jen.Op("&").Id(VarBody))

			case "application/octet-stream":
				return jen.Add(jen.Var().Defs(
					jen.Id(ParamBuf).Interface(),
					jen.Id(ParamOk).Bool(),
					jen.Id(ParamReadErr).Error(),
				),
					jen.Line(),
					jen.If(
						jen.List(jen.Id(ParamBuf), jen.Id(ParamReadErr)).Op("=").Qual("io", "ReadAll").Call(jen.Id(VarR).Dot("Body")),
						jen.Id(ParamReadErr).Op("==").Nil(),
					).Block(
						jen.If(
							jen.List(jen.Id(VarBody), jen.Id(ParamOk)).Op("=").Id(ParamBuf).Assert(jen.Qual(generator.config.ComponentsPackage, name)),
							jen.Op("!").Id(ParamOk),
						).Block(
							jen.Id(ParamDecodeErr).Op("=").Qual("errors", "New").Call(jen.Lit("body is not []byte")),
						),
					))
			default:
				return jen.Id(ParamDecodeErr).Op("=").Qual(PackageEncodingJSON, "NewDecoder").Call(jen.Id(VarR).Dot("Body")).Dot("Decode").Call(jen.Op("&").Id(VarBody))
			}
		}()).
		Add(jen.Line()).
		Add(jen.If(jen.Id(ParamDecodeErr).Op("!=").Id(ParamNil)).Block(
			jen.Id(VarRequest).Dot(FieldProcessingResult).Op("=").Qual(generator.config.ComponentsPackage, "RequestProcessingResult").Values(jen.Id(ParamError).Op(":").Id(ParamDecodeErr),
				jen.Id(ParamType).Op(":").Id("BodyUnmarshalFailed")),
			jen.If(jen.Id(VarRouter).Dot(VarHooks).Dot("RequestBodyUnmarshalFailed").Op("!=").Id(ParamNil)).Block(
				jen.Id(VarRouter).Dot(VarHooks).Dot("RequestBodyUnmarshalFailed").Call(
					jen.Id(VarR),
					jen.Lit(wrapperName),
					jen.Id(VarRequest).Dot(FieldProcessingResult))),
			jen.Line().Return())).
		Add(jen.Line()).
		Add(jen.Id(VarRequest).Dot(InBody).Op("=").Id(VarBody)).
		Add(jen.Line())

	return result
}