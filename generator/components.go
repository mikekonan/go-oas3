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
		// Check if we need null handling (has required fields)
		hasRequiredFields := len(schema.Required) > 0
		var codeBlocks []jen.Code

		if hasRequiredFields {
			// Generate private helper struct with pointers for null detection
			helperName := generator.normalizer.decapitalize(name)
			helperStruct := jen.Type().Id(helperName).Struct(generator.typeProperties(name, schema, true)...)
			codeBlocks = append(codeBlocks, helperStruct)
		}

		// Generate public struct with regular fields
		publicStruct := jen.Type().Id(name).Struct(generator.typeProperties(name, schema, false)...)
		codeBlocks = append(codeBlocks, publicStruct)

		if hasRequiredFields {
			// Generate UnmarshalJSON method for null handling
			unmarshalMethod := generator.generateUnmarshalJSON(name, schema)
			if unmarshalMethod != nil {
				codeBlocks = append(codeBlocks, unmarshalMethod)
			}
		}

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

			// For null-handled required fields, don't add Required rule since UnmarshalJSON handles it
			skipRequiredRule := hasRequiredFields && isRequired
			rules := generator.fieldValidationRuleFromSchema(name[0:1], fieldName, propertySchema, isRequired && !skipRequiredRule)
			if len(rules) > 0 {
				validationRules = append(validationRules, rules...)
			}
		}

		validationFunc := generator.validationFuncFromRules(name[0:1], name, validationRules, schema)
		if validationFunc != nil {
			codeBlocks = append(codeBlocks, validationFunc)
		}

		// Join all code blocks with proper spacing
		if len(codeBlocks) == 1 {
			return codeBlocks[0]
		}
		
		var result []jen.Code
		for i, block := range codeBlocks {
			if i > 0 {
				result = append(result, jen.Line(), jen.Line())
			}
			result = append(result, block)
		}
		
		return jen.Add(result...)
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

		// Determine when to use pointers:
		// - If pointersForRequired is true (helper struct), use pointers for required fields that don't already have x-go-pointer
		// - For optional fields, use pointers if they have x-go-pointer or x-go-omitempty
		usePointer := false
		if pointersForRequired && isRequired && !generator.typee.getXGoPointer(propSchema.Value) {
			// Helper struct: required fields get pointers for null detection
			usePointer = true
		} else if !pointersForRequired && (!isRequired || generator.typee.getXGoPointer(propSchema.Value)) {
			// Public struct: only optional fields or explicitly marked fields get pointers
			usePointer = generator.typee.getXGoPointer(propSchema.Value)
		}

		// Fill Go type
		generator.typee.fillGoType(field, typeName, fieldName, propSchema, usePointer, false)
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

	// Check if this enum should be aliased to an external type instead of generating constants
	if generator.shouldUseExternalTypeAlias(v) {
		return generator.generateExternalTypeAlias(name, schema)
	}

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

// shouldUseExternalTypeAlias determines if an enum schema should use a type alias to an external type
func (generator *Generator) shouldUseExternalTypeAlias(schema *openapi3.Schema) bool {
	// Check if schema has x-go-type extension (explicit external type mapping)
	if generator.typee.hasXGoType(schema) {
		return true
	}
	
	// Check for well-known formats that map to external types
	if schema.Type != nil && schema.Type.Is(TypeString) && schema.Format != "" {
		switch schema.Format {
		case FormatISO3166Alpha2, FormatISO3166Alpha3, FormatISO4217CurrencyCode:
			return true
		}
	}
	
	return false
}

// generateExternalTypeAlias generates a type alias to an external type
func (generator *Generator) generateExternalTypeAlias(name string, schema *openapi3.SchemaRef) jen.Code {
	if schema == nil || schema.Value == nil {
		return jen.Null()
	}

	v := schema.Value
	typeAlias := jen.Type().Id(name).Op("=")
	
	// Check if there's an explicit x-go-type mapping
	if pkg, typeName, ok := generator.typee.getXGoType(v); ok {
		if pkg == "" {
			typeAlias.Id(typeName)
		} else {
			typeAlias.Qual(pkg, typeName)
		}
		return typeAlias
	}
	
	// Handle well-known formats
	if v.Type != nil && v.Type.Is(TypeString) && v.Format != "" {
		switch v.Format {
		case FormatISO3166Alpha2:
			typeAlias.Qual("github.com/mikekonan/go-types/v2/country", "Alpha2Code")
		case FormatISO3166Alpha3:
			typeAlias.Qual("github.com/mikekonan/go-types/v2/country", "Alpha3Code")
		case FormatISO4217CurrencyCode:
			typeAlias.Qual("github.com/mikekonan/go-types/v2/currency", "Code")
		default:
			// Fallback to string if format is unknown
			typeAlias.String()
		}
		return typeAlias
	}
	
	// Fallback - shouldn't reach here if shouldUseExternalTypeAlias returned true
	typeAlias.String()
	return typeAlias
}

// generateEnumValidation generates validation method for enum types
func (generator *Generator) generateEnumValidation(name string, schema *openapi3.SchemaRef) jen.Code {
	if schema == nil || schema.Value == nil {
		return jen.Null()
	}

	receiverName := "c"
	if name != "" {
		receiverName = strings.ToLower(name[:1])
	}
	
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

// generateUnmarshalJSON generates UnmarshalJSON method with null detection for required fields
func (generator *Generator) generateUnmarshalJSON(typeName string, schema *openapi3.Schema) jen.Code {
	if len(schema.Required) == 0 {
		return jen.Null()
	}

	helperName := generator.normalizer.decapitalize(typeName)
	receiverName := strings.ToLower(typeName[:1])

	// Create the method signature
	method := jen.Func().Params(
		jen.Id(receiverName).Op("*").Id(typeName),
	).Id("UnmarshalJSON").Params(
		jen.Id("data").Index().Byte(),
	).Error()

	// Build method body
	methodBody := []jen.Code{
		// var value helperType
		jen.Var().Id("value").Id(helperName),
		
		// if err := json.Unmarshal(data, &value); err != nil { return err }
		jen.If(
			jen.Id("err").Op(":=").Qual("encoding/json", "Unmarshal").Call(
				jen.Id("data"), jen.Op("&").Id("value"),
			).Op(";").Id("err").Op("!=").Nil(),
		).Block(
			jen.Return().Id("err"),
		),
	}

	// Add null checks for required fields
	for _, reqField := range schema.Required {
		fieldName := generator.normalizer.normalize(reqField)
		propSchema := schema.Properties[reqField]
		
		// Skip fields that already have pointer types or custom handling
		if propSchema != nil && propSchema.Value != nil && generator.typee.getXGoPointer(propSchema.Value) {
			continue
		}
		
		// Add null check
		nullCheck := jen.If(
			jen.Id("value").Dot(fieldName).Op("==").Nil(),
		).Block(
			jen.Return().Qual(PackageFmt, MethodErrorf).Call(
				jen.Lit("field '%s' is required but was null or missing"), 
				jen.Lit(fieldName),
			),
		)
		
		methodBody = append(methodBody, nullCheck)
	}

	// Copy values from helper to public struct with proper handling
	for propName, propSchema := range schema.Properties {
		fieldName := generator.normalizer.normalize(propName)
		isRequired := false
		for _, reqField := range schema.Required {
			if reqField == propName {
				isRequired = true
				break
			}
		}

		if isRequired && !generator.typee.getXGoPointer(propSchema.Value) {
			// For required non-pointer fields, dereference the pointer and apply trimming if needed
			assignment := jen.Id(receiverName).Dot(fieldName).Op("=")
			
			// Check if field should be trimmed
			if propSchema.Value != nil && generator.getXGoStringTrimmable(propSchema) {
				assignment.Qual(PackageStrings, MethodTrimSpace).Call(
					jen.Op("*").Id("value").Dot(fieldName),
				)
			} else {
				assignment.Op("*").Id("value").Dot(fieldName)
			}
			
			methodBody = append(methodBody, assignment)
		} else {
			// For optional fields, handle pointer assignment
			assignment := jen.Id(receiverName).Dot(fieldName).Op("=").Id("value").Dot(fieldName)
			methodBody = append(methodBody, assignment)
		}
	}

	// Return nil
	methodBody = append(methodBody, jen.Return().Nil())

	return method.Block(methodBody...)
}