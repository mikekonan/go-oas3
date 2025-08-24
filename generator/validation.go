package generator

import (
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
)

// validationFuncFromRules generates validation functions from validation rules
func (generator *Generator) validationFuncFromRules(receiverName string, name string, rules []jen.Code, schema *openapi3.Schema) jen.Code {
	if schema != nil && generator.typee.getXGoSkipValidation(schema) {
		return jen.Null()
	}

	block := jen.Return().Id("nil")
	if len(rules) > 0 {
		params := []jen.Code{jen.Op("&").Id(receiverName)}
		params = append(params, rules...)
		block = jen.Return().Qual("github.com/go-ozzo/ozzo-validation/v4", "ValidateStruct").Call(params...)
	}

	return jen.Func().Params(
		jen.Id(receiverName).Id(name)).Id(MethodValidate).Params().Params(
		jen.Id("error")).Block(block)
}

// fieldValidationRuleFromSchema generates field validation rules from OpenAPI schema
func (generator *Generator) fieldValidationRuleFromSchema(receiverName string, propertyName string, schema *openapi3.SchemaRef, required bool) []jen.Code {
	var fieldRules []jen.Code
	
	if schema == nil || schema.Value == nil {
		return fieldRules
	}
	
	v := schema.Value

	if generator.typee.getXGoSkipValidation(v) {
		return fieldRules
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
			lengthRule := jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(params...)
			fieldRules = append(fieldRules, lengthRule)
		}
		
		// Handle regex validation
		regexPattern := generator.getXGoRegex(&openapi3.SchemaRef{Value: v})
		if regexPattern != "" {
			var params = []jen.Code{jen.Op("&").Id(receiverName).Dot(propertyName)}
			if required {
				params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Required"))
			}
			
			regexVarName := generator.normalizer.decapitalize(generator.normalizer.normalize(propertyName) + SuffixRegex)
			params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Match").Call(jen.Id(regexVarName)))
			
			regexRule := jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(params...)
			fieldRules = append(fieldRules, regexRule)
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
			fieldRules = append(fieldRules, trimRule)
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
			numericRule := jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(params...)
			fieldRules = append(fieldRules, numericRule)
		}
	} else if v.Type != nil && v.Type.Is(TypeArray) {
		var rules []jen.Code
		
		// Always add Length validation for arrays based on minItems/maxItems
		minItems := int(v.MinItems)
		maxItems := 1000000 // Large default max
		if v.MaxItems != nil {
			maxItems = int(*v.MaxItems)
		}
		
		rules = append(rules, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Length").Call(
			jen.Lit(minItems), jen.Lit(maxItems)))
		
		// Always generate validation for arrays if we have any constraints
		if len(rules) > 0 {
			var params = []jen.Code{jen.Op("&").Id(receiverName).Dot(propertyName)}
			
			// Add Required rule for arrays with minItems > 0
			// This is needed because Length validation doesn't apply to empty/nil arrays in ozzo-validation
			if minItems > 0 {
				params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Required"))
			}
			
			// Also add Required if the field is marked as required (just in case)
			if required {
				// Only add if we haven't already added it
				if minItems == 0 {
					params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Required"))
				}
			}
			
			params = append(params, rules...)
			arrayRule := jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(params...)
			fieldRules = append(fieldRules, arrayRule)
		}
	}

	return fieldRules
}