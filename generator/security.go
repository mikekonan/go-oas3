package generator

import (
	"github.com/ahmetb/go-linq"
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cast"
)

// wrapperSecurity generates security wrapper code for operations
func (generator *Generator) wrapperSecurity(name string, operation *openapi3.Operation) jen.Code {
	if operation == nil || operation.Security == nil || len(*operation.Security) == 0 {
		return jen.Null()
	}

	// Skip security check if extension is set
	if generator.typee.getXGoSkipSecurityCheck(operation) {
		return jen.Null()
	}

	var securityBlocks []jen.Code

	for _, securityRequirement := range *operation.Security {
		if len(securityRequirement) == 0 {
			// Empty security requirement means no authentication required
			continue
		}

		for schemeName := range securityRequirement {
			
			securityBlock := jen.Line().For(jen.List(jen.Id("_"), jen.Id("processor")).Op(":=").Range().Id("router").Dot("processors")).Block(
				jen.If(jen.Id("processor").Dot("scheme").Op("==").Id("SecurityScheme"+generator.normalizer.titleCase(schemeName))).Block(
					jen.List(jen.Id("name"), jen.Id("value"), jen.Id("found")).Op(":=").Id("processor").Dot("extract").Call(jen.Id("r")),
					jen.If(jen.Op("!").Id("found")).Block(
						jen.Id("request").Dot(FieldProcessingResult).Op("=").Id("RequestProcessingResult").Values(
							jen.Id("error").Op(":").Qual(PackageFmt, MethodErrorf).Call(jen.Lit("security scheme not found")),
							jen.Id("typee").Op(":").Id("SecurityParseFailed")),
						jen.If(jen.Id("router").Dot("hooks").Dot("RequestSecurityParseFailed").Op("!=").Id("nil")).Block(
							jen.Id("router").Dot("hooks").Dot("RequestSecurityParseFailed").Call(
								jen.Id("r"),
								jen.Lit(name),
								jen.Id("request").Dot(FieldProcessingResult))),
						jen.Return(),
					),
					jen.Line(),
					jen.If(jen.Id("err").Op(":=").Id("processor").Dot("handle").Call(jen.Id("r"), jen.Id("processor").Dot("scheme"), jen.Id("name"), jen.Id("value")), jen.Id("err").Op("!=").Id("nil")).Block(
						jen.Id("request").Dot(FieldProcessingResult).Op("=").Id("RequestProcessingResult").Values(
							jen.Id("error").Op(":").Id("err"),
							jen.Id("typee").Op(":").Id("SecurityCheckFailed")),
						jen.If(jen.Id("router").Dot("hooks").Dot("RequestSecurityCheckFailed").Op("!=").Id("nil")).Block(
							jen.Id("router").Dot("hooks").Dot("RequestSecurityCheckFailed").Call(
								jen.Id("r"),
								jen.Lit(name),
								jen.String().Call(jen.Id("processor").Dot("scheme")),
								jen.Id("request").Dot(FieldProcessingResult))),
						jen.Return(),
					),
					jen.Line(),
					jen.If(jen.Id("router").Dot("hooks").Dot("RequestSecurityCheckCompleted").Op("!=").Id("nil")).Block(
						jen.Id("router").Dot("hooks").Dot("RequestSecurityCheckCompleted").Call(
							jen.Id("r"),
							jen.Lit(name),
							jen.String().Call(jen.Id("processor").Dot("scheme")))),
					jen.Break(),
				),
			)

			securityBlocks = append(securityBlocks, securityBlock)
		}
	}

	if len(securityBlocks) == 0 {
		return jen.Null()
	}

	if len(securityBlocks) == 1 {
		return securityBlocks[0]
	}
	return jen.Add(securityBlocks...)
}

// securitySchemas generates security scheme types and processors
func (generator *Generator) securitySchemas(swagger *openapi3.T) jen.Code {
	if swagger.Components == nil || len(swagger.Components.SecuritySchemes) == 0 {
		return jen.Null()
	}

	code := jen.Type().Id("SecurityScheme").Id("string").Line().Line()

	// Generate security scheme constants
	var consts []jen.Code
	linq.From(swagger.Components.SecuritySchemes).
		SelectT(func(kv linq.KeyValue) jen.Code {
			name := generator.normalizer.titleCase(cast.ToString(kv.Key))
			return jen.Id("SecurityScheme" + name).Id("SecurityScheme").Op("=").Lit(name)
		}).
		ToSlice(&consts)

	code = code.Const().Defs(consts...).Line().Line()

	// Generate security processor struct
	code = code.Line().Line().
		Type().Id("securityProcessor").Struct(
		jen.Id("scheme").Id("SecurityScheme"),
		jen.Id("extract").Func().Params(jen.Id("r").Op("*").Qual(PackageNetHTTP, "Request")).
			Params(jen.Id("string"), jen.Id("string"), jen.Id("bool")),
		jen.Id("handle").Func().Params(jen.Id("r").Op("*").Qual(PackageNetHTTP, "Request"),
			jen.Id("scheme").Id("SecurityScheme"), jen.Id("name").Id("string"),
			jen.Id("value").Id("string")).Params(
			jen.Id("error")))

	// Generate security extractors
	var extractorsHeadersFuncs []jen.Code
	linq.From(swagger.Components.SecuritySchemes).
		SelectT(func(kv linq.KeyValue) jen.Code {
			name := generator.normalizer.normalize(cast.ToString(kv.Key))
			schema := kv.Value.(*openapi3.SecuritySchemeRef)

			if schema.Value.Type == "http" {
				ifStatement := jen.Null()
				assignment := jen.Null()
				if schema.Value.Scheme == "bearer" {
					ifStatement = ifStatement.Op("!").Qual(PackageStrings, "HasPrefix").Call(jen.Id("value"), jen.Lit("Bearer "))
					assignment = assignment.Id("value").Op("=").Id("value").Index(jen.Lit(7), jen.Empty())
				} else {
					ifStatement = ifStatement.Op("!").Qual(PackageStrings, "HasPrefix").Call(jen.Id("value"), jen.Lit("Basic "))
					assignment = assignment.Id("value").Op("=").Id("value").Index(jen.Lit(6), jen.Empty())
				}

				return jen.Line().Id("SecurityScheme"+generator.normalizer.titleCase(name)).Op(":").Func().Params(
					jen.Id("r").Op("*").Qual(PackageNetHTTP, "Request")).Params(jen.Id("string"), jen.Id("string"),
					jen.Id("bool")).Block(
					jen.Id("value").Op(":=").Id("r").Dot("Header").Dot("Get").Call(jen.Lit("Authorization")).Line(),
					jen.If(ifStatement).Block(jen.Return().List(jen.Lit(""), jen.Lit(""), jen.Id("false"))).Line(),
					assignment.Line(),
					jen.Return().List(jen.Lit("Authorization"), jen.Id("value"), jen.Id("value").Op("!=").Lit("")))
			}

			if schema.Value.Type == "apiKey" {
				switch schema.Value.In {
				case "header":
					return jen.Line().Id("SecurityScheme"+generator.normalizer.titleCase(name)).Op(":").Func().Params(
						jen.Id("r").Op("*").Qual(PackageNetHTTP, "Request")).Params(jen.Id("string"), jen.Id("string"),
						jen.Id("bool")).Block(
						jen.Id("value").Op(":=").Id("r").Dot("Header").Dot("Get").Call(jen.Lit(schema.Value.Name)),
						jen.Return().List(jen.Lit(schema.Value.Name), jen.Id("value"), jen.Id("value").Op("!=").Lit("")))
				case "query":
					return jen.Line().Id("SecurityScheme"+generator.normalizer.titleCase(name)).Op(":").Func().Params(
						jen.Id("r").Op("*").Qual(PackageNetHTTP, "Request")).Params(jen.Id("string"), jen.Id("string"),
						jen.Id("bool")).Block(
						jen.Id("value").Op(":=").Id("r").Dot("URL").Dot("Query").Call().Dot("Get").Call(jen.Lit(schema.Value.Name)),
						jen.Return().List(jen.Lit(schema.Value.Name), jen.Id("value"), jen.Id("value").Op("!=").Lit("")))
				case "cookie":
					return jen.Line().Id("SecurityScheme"+generator.normalizer.titleCase(name)).Op(":").Func().Params(
						jen.Id("r").Op("*").Qual(PackageNetHTTP, "Request")).Params(jen.Id("string"), jen.Id("string"),
						jen.Id("bool")).Block(
						jen.List(jen.Id("cookie"), jen.Id("err")).Op(":=").Id("r").Dot("Cookie").Call(jen.Lit(schema.Value.Name)),
						jen.If(jen.Id("err").Op("!=").Id("nil")).Block(jen.Return().List(jen.Lit(""), jen.Lit(""), jen.Id("false"))),
						jen.Return().List(jen.Lit(schema.Value.Name), jen.Id("cookie").Dot("Value"), jen.Id("true")))
				}
			}

			return jen.Null()
		}).
		WhereT(func(code jen.Code) bool { return code != jen.Null() }).
		ToSlice(&extractorsHeadersFuncs)

	// Generate the extractor map with inline functions
	if len(extractorsHeadersFuncs) > 0 {
		code = code.Line().Line().Var().Id("SecuritySchemeExtractors").Op("=").Map(jen.Id("SecurityScheme")).Func().Params(
			jen.Id("r").Op("*").Qual(PackageNetHTTP, "Request")).Params(jen.Id("string"), jen.Id("string"), jen.Id("bool")).
			Values(extractorsHeadersFuncs...)
	}

	return code
}