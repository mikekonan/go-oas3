package generator

import (
	"strings"

	"github.com/ahmetb/go-linq"
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cast"
)

// components generates OpenAPI component types and enums
func (generator *Generator) components(swagger *openapi3.T) jen.Code {
	var componentsResult []jen.Code

	// Check if components exist
	if swagger.Components == nil || swagger.Components.Schemas == nil {
		return jen.Empty()
	}

	// Generate regular components (non-enum schemas)
	linq.From(swagger.Components.Schemas).
		WhereT(func(kv linq.KeyValue) bool { return len(kv.Value.(*openapi3.SchemaRef).Value.Enum) == 0 }). //filter enums
		SelectT(func(kv linq.KeyValue) jen.Code {
			schemaRef := kv.Value.(*openapi3.SchemaRef)
			return generator.componentFromSchema(cast.ToString(kv.Key), schemaRef)
		}).
		ToSlice(&componentsResult)

	// Generate components from paths (inline schemas)
	var componentsFromPathsResult []jen.Code
	linq.From(swagger.Paths.Map()).
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
						name := generator.normalizer.normalizeOperationName(path, cast.ToString(kv.Key))
						operation := kv.Value.(*openapi3.Operation)

						return linq.From(operation.RequestBody.Value.Content).
							WhereT(func(kv linq.KeyValue) bool {
								return kv.Value.(*openapi3.MediaType).Schema.Ref == ""
							}).
							SelectT(func(kv linq.KeyValue) linq.KeyValue {
								contentType := cast.ToString(kv.Key)
								requestBodyName := name + generator.normalizer.contentType(contentType) + SuffixRequestBody
								mediaType := kv.Value.(*openapi3.MediaType)
								return linq.KeyValue{
									Key:   requestBodyName,
									Value: generator.componentFromSchema(requestBodyName, mediaType.Schema),
								}
							})
					}).
				ForEachT(func(componentKv linq.KeyValue) {
					componentsByName[cast.ToString(componentKv.Key)] = componentKv.Value.(jen.Code)
				})

			return linq.From(componentsByName).SelectT(func(kv linq.KeyValue) jen.Code {
				return kv.Value.(jen.Code)
			})
		}).
		ToSlice(&componentsFromPathsResult)

	// Combine all components
	allComponents := append(componentsResult, componentsFromPathsResult...)
	allComponents = append(allComponents, generator.enums(swagger))
	
	if len(allComponents) == 0 {
		return jen.Null()
	}

	allComponents = generator.normalizer.doubleLineAfterEachElement(allComponents...)
	return jen.Add(allComponents...)
}

// componentFromSchema generates a Go struct from an OpenAPI schema
func (generator *Generator) componentFromSchema(name string, parentSchema *openapi3.SchemaRef) jen.Code {
	if parentSchema == nil || parentSchema.Value == nil {
		return jen.Null()
	}

	schema := parentSchema.Value
	name = generator.normalizer.normalize(name)

	// Handle enum types
	if len(schema.Enum) > 0 {
		return generator.enumFromSchema(name, parentSchema)
	}

	// Handle object types (explicit or implicit with properties)
	if (schema.Type != nil && schema.Type.Is(TypeObject)) || (len(schema.Properties) > 0) {
		structType := jen.Type().Id(name).Struct(generator.typeProperties(name, schema, false)...)

		// Generate validation function if needed
		var validationRules []jen.Code
		for propertyName, propertySchema := range schema.Properties {
			fieldName := generator.normalizer.normalize(propertyName)
			isRequired := false
			for _, reqField := range schema.Required {
				if reqField == propertyName {
					isRequired = true
					break
				}
			}

			rules := generator.fieldValidationRuleFromSchema(name[0:1], fieldName, propertySchema, isRequired)
			if len(rules) > 0 {
				validationRules = append(validationRules, rules...)
			}
		}

		validationFunc := generator.validationFuncFromRules(name[0:1], name, validationRules, schema)
		if validationFunc != nil {
			return jen.Add(structType, jen.Line(), jen.Line(), validationFunc)
		}

		return structType
	}

	// Handle array types
	if schema.Type != nil && schema.Type.Is(TypeArray) && schema.Items != nil {
		arrayType := jen.Type().Id(name)
		generator.typee.fillGoType(arrayType, "", name, parentSchema, false, true)
		return arrayType
	}

	// Handle simple types (aliases)
	if schema.Type != nil {
		aliasType := jen.Type().Id(name)
		generator.typee.fillGoType(aliasType, "", name, parentSchema, false, true)
		return aliasType
	}

	return jen.Null()
}

// typeProperties generates struct properties from schema properties
func (generator *Generator) typeProperties(typeName string, schema *openapi3.Schema, pointersForRequired bool) (parameters []jen.Code) {
	for propName, propSchema := range schema.Properties {
		fieldName := generator.normalizer.normalize(propName)
		field := jen.Id(fieldName)

		// Check if field is required
		isRequired := false
		for _, reqField := range schema.Required {
			if reqField == propName {
				isRequired = true
				break
			}
		}

		// Fill Go type
		generator.typee.fillGoType(field, typeName, fieldName, propSchema, !isRequired && pointersForRequired, false)
		generator.typee.fillJsonTag(field, propSchema, propName)

		parameters = append(parameters, field)
	}

	// Handle additional properties
	if schema.AdditionalProperties.Schema != nil {
		generator.typee.fillAdditionalProperties(jen.Add(parameters...), schema)
	}

	return parameters
}

// enums generates all enum types from the OpenAPI specification
func (generator *Generator) enums(swagger *openapi3.T) jen.Code {
	var enumsResult []jen.Code

	// Check if components exist
	if swagger.Components == nil || swagger.Components.Schemas == nil {
		return jen.Null()
	}

	// Generate enums from root-level components
	for name, schemaRef := range swagger.Components.Schemas {
		if len(schemaRef.Value.Enum) > 0 {
			result := generator.enumFromSchema(name, schemaRef)
			if result != jen.Null() {
				enumsResult = append(enumsResult, result)
			}
		}
	}

	// Generate enums from nested properties in schemas
	var nestedEnumsResult []jen.Code
	linq.From(swagger.Components.Schemas).
		WhereT(func(kv linq.KeyValue) bool { 
			schema := kv.Value.(*openapi3.SchemaRef).Value
			return len(schema.Enum) == 0 && len(schema.Properties) > 0 // Only object types, not root enums
		}).
		SelectManyT(func(kv linq.KeyValue) linq.Query {
			schema := kv.Value.(*openapi3.SchemaRef).Value
			var nestedEnums []jen.Code
			
			for propName, propSchema := range schema.Properties {
				if propSchema.Value != nil && len(propSchema.Value.Enum) > 0 {
					enumName := generator.normalizer.normalize(propName)
					nestedEnums = append(nestedEnums, generator.enumFromSchema(enumName, propSchema))
				}
			}
			
			return linq.From(nestedEnums)
		}).
		ToSlice(&nestedEnumsResult)
	
	// Combine root-level and nested enums
	enumsResult = append(enumsResult, nestedEnumsResult...)

	if len(enumsResult) == 0 {
		return jen.Null()
	}

	enumsResult = generator.normalizer.doubleLineAfterEachElement(enumsResult...)
	return jen.Add(enumsResult...)
}

// enumFromSchema generates enum type and constants from schema
func (generator *Generator) enumFromSchema(name string, schema *openapi3.SchemaRef) jen.Code {
	if schema == nil || schema.Value == nil || len(schema.Value.Enum) == 0 {
		return jen.Null()
	}

	name = generator.normalizer.normalize(name)
	v := schema.Value

	// Determine base type
	baseType := jen.String()
	if v.Type != nil && v.Type.Is(TypeInteger) {
		baseType = jen.Int()
	} else if v.Type != nil && v.Type.Is(TypeNumber) {
		baseType = jen.Float64()
	}

	// Generate type definition
	typeDecl := jen.Type().Id(name).Add(baseType)

	// Generate constants
	var consts []jen.Code
	for _, enumValue := range v.Enum {
		constName := name + generator.normalizer.normalize(cast.ToString(enumValue))
		
		var constValue jen.Code
		if v.Type != nil && v.Type.Is(TypeInteger) {
			constValue = jen.Lit(cast.ToInt(enumValue))
		} else if v.Type != nil && v.Type.Is(TypeNumber) {
			constValue = jen.Lit(cast.ToFloat64(enumValue))
		} else {
			constValue = jen.Lit(cast.ToString(enumValue))
		}

		consts = append(consts, jen.Id(constName).Id(name).Op("=").Add(constValue))
	}

	constDecl := jen.Const().Defs(consts...)

	// Generate validation method
	validationMethod := generator.generateEnumValidation(name, schema)

	return jen.Add(typeDecl, jen.Line(), jen.Line(), constDecl, jen.Line(), jen.Line(), validationMethod)
}

// generateEnumValidation generates validation method for enum types
func (generator *Generator) generateEnumValidation(name string, schema *openapi3.SchemaRef) jen.Code {
	if schema == nil || schema.Value == nil {
		return jen.Null()
	}

	receiverName := strings.ToLower(name[:1])
	
	// Create validation cases
	var cases []jen.Code
	for _, enumValue := range schema.Value.Enum {
		constName := name + generator.normalizer.normalize(cast.ToString(enumValue))
		cases = append(cases, jen.Id(constName))
	}

	// Generate switch statement
	switchStmt := jen.Switch(jen.Id(receiverName)).BlockFunc(func(g *jen.Group) {
		for _, c := range cases {
			g.Case(c).Block(jen.Return().Nil())
		}
		g.Default().Block(
			jen.Return().Qual(PackageFmt, MethodErrorf).Call(
				jen.Lit(ErrorInvalidEnum),
				jen.Id(receiverName),
			),
		)
	})

	return jen.Func().Params(jen.Id(receiverName).Id(name)).Id(MethodValidate).Params().Error().Block(switchStmt)
}