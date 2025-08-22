package generator

import (
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/mikekonan/go-oas3/configurator"
)

// Generator coordinates the generation of OpenAPI-based Go code
type Generator struct {
	normalizer *Normalizer          `di.inject:"normalizer"`
	typee      *Type                `di.inject:"typeFiller"`
	config     *configurator.Config `di.inject:"config"`

	// optimize code generator for regexp
	useRegex map[string]string
}

// Result contains the generated code files
type Result struct {
	ComponentsCode *jen.File
	RouterCode     *jen.File
	SpecCode       *jen.File
}

// Generate generates all code from the OpenAPI specification
func (generator *Generator) Generate(swagger *openapi3.T) *Result {
	componentsAdditionalVars, parametersAdditionalVars := generator.additionalConstants(swagger)

	componentsCode := jen.Null().Add(componentsAdditionalVars, generator.components(swagger))
	routerCode := jen.Null().
		Add(parametersAdditionalVars...).Line().
		Add(generator.wrappers(swagger)).Line().
		Add(generator.requestResponseBuilders(swagger)).Line().
		Add(generator.securitySchemas(swagger))

	return &Result{
		ComponentsCode: generator.file(componentsCode, generator.config.ComponentsPackage),
		RouterCode:     generator.file(routerCode, generator.config.Package),
		SpecCode:       generator.file(generator.specCode(swagger), generator.config.Package),
	}
}

// specCode generates the OpenAPI specification code
func (generator *Generator) specCode(swagger *openapi3.T) jen.Code {
	specBytes, err := swagger.MarshalJSON()
	if err != nil {
		PanicOperationError("OpenAPI Spec Marshal", err, map[string]interface{}{
			"operation": "marshaling OpenAPI specification to JSON",
			"context":   "spec code generation",
		})
	}

	return jen.Var().Id("OpenAPISpec").Op("=").Index().Byte().Call(jen.Lit(string(specBytes)))
}