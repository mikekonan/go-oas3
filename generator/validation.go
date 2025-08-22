package generator

import (
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
)

// validationFuncFromRules generates validation functions from validation rules
func (generator *Generator) validationFuncFromRules(receiverName string, name string, rules []jen.Code, schema *openapi3.Schema) jen.Code {
	if schema != nil && generator.typee.getXGoSkipValidation(schema) {
		return nil
	}

	block := jen.Return().Id("nil")
	if len(rules) > 0 {
		params := append([]jen.Code{jen.Op("&").Id(receiverName)}, rules...)
		block = jen.Return().Qual("github.com/go-ozzo/ozzo-validation/v4", "ValidateStruct").Call(params...)
	}

	return jen.Func().Params(
		jen.Id(receiverName).Id(name)).Id(MethodValidate).Params().Params(
		jen.Id("error")).Block(block)
}

// fieldValidationRuleFromSchema generates field validation rules from OpenAPI schema
func (generator *Generator) fieldValidationRuleFromSchema(receiverName string, propertyName string, schema *openapi3.SchemaRef, required bool) jen.Code {
	var fieldRule jen.Code
	v := schema.Value

	if generator.typee.getXGoSkipValidation(v) {
		return fieldRule
	}

	if v.Type != nil && v.Type.Is(TypeString) {
		if v.MaxLength != nil || v.MinLength > 0 {
			var maxLength uint64
			if v.MaxLength != nil {
				maxLength = *v.MaxLength
			}
			var params = []jen.Code{jen.Op("&").Id(receiverName).Dot(propertyName)}
			
			if v.MinLength > 0 && required {
				params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Required"))
			} else if v.MinLength > 0 {
				params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Skip").Dot("When").Call(
					jen.Id(receiverName).Dot(propertyName).Op("==").Lit("")))
			}
			
			params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "RuneLength").Call(
				jen.Lit(int(v.MinLength)), jen.Lit(int(maxLength))))
			fieldRule = jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(params...)
		}
		
		// Handle regex validation
		regexPattern := generator.getXGoRegex(&openapi3.SchemaRef{Value: v})
		if regexPattern != "" {
			var params = []jen.Code{jen.Op("&").Id(receiverName).Dot(propertyName)}
			if required {
				params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Required"))
			}
			
			regexVarName := generator.normalizer.normalize(propertyName) + SuffixRegex
			params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Match").Call(jen.Id(regexVarName)))
			
			regexRule := jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(params...)
			if fieldRule != nil {
				fieldRule = jen.Add(fieldRule, jen.Line(), regexRule)
			} else {
				fieldRule = regexRule
			}
		}
		
		// Handle string trimming validation
		if generator.getXGoStringTrimmable(&openapi3.SchemaRef{Value: v}) {
			var params = []jen.Code{jen.Op("&").Id(receiverName).Dot(propertyName)}
			params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "By").Call(
				jen.Func().Params(jen.Id("value").Interface()).Error().Block(
					jen.If(jen.Id("str").Op(",").Id("ok").Op(":=").Id("value").Assert(jen.String()).Op(";").Id("ok")).Block(
						jen.Id("trimmed").Op(":=").Qual(PackageStrings, MethodTrimSpace).Call(jen.Id("str")),
						jen.If(jen.Id("str").Op("!=").Id("trimmed")).Block(
							jen.Return().Qual(PackageFmt, MethodErrorf).Call(jen.Lit("value should be trimmed")),
						),
					),
					jen.Return().Id("nil"),
				),
			))
			
			trimRule := jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(params...)
			if fieldRule != nil {
				fieldRule = jen.Add(fieldRule, jen.Line(), trimRule)
			} else {
				fieldRule = trimRule
			}
		}
		
	} else if v.Type != nil && (v.Type.Is(TypeInteger) || v.Type.Is(TypeNumber)) {
		var rules []jen.Code
		
		if v.Min != nil {
			min := jen.Lit(*v.Min)
			if v.Type != nil && v.Type.Is(TypeInteger) {
				min = jen.Lit(int(*v.Min))
			}
			r := jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Min").Call(min)
			if v.ExclusiveMin {
				r.Dot("Exclusive").Call()
			}
			rules = append(rules, r)
		}
		
		if v.Max != nil {
			max := jen.Lit(*v.Max)
			if v.Type != nil && v.Type.Is(TypeInteger) {
				max = jen.Lit(int(*v.Max))
			}
			r := jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Max").Call(max)
			if v.ExclusiveMax {
				r.Dot("Exclusive").Call()
			}
			rules = append(rules, r)
		}
		
		if len(rules) > 0 {
			var params = []jen.Code{jen.Op("&").Id(receiverName).Dot(propertyName)}
			if required {
				params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Required"))
			}
			params = append(params, rules...)
			fieldRule = jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(params...)
		}
	}

	return fieldRule
}