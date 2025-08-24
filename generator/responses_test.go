package generator

import (
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mikekonan/go-oas3/configurator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupResponsesTestSimple() *Generator {
	return &Generator{
		normalizer: &Normalizer{},
		typee: &Type{
			normalizer: &Normalizer{},
			config: &configurator.Config{
				ComponentsPackage: "components",
				Package:           "api",
			},
		},
		config: &configurator.Config{
			ComponentsPackage: "components",
			Package:           "api",
		},
		useRegex: make(map[string]string),
	}
}

func TestGenerator_requestResponseBuilders_Simple(t *testing.T) {
	generator := setupResponsesTestSimple()

	// Create simple swagger spec
	swagger := &openapi3.T{}
	paths := &openapi3.Paths{}
	responses := &openapi3.Responses{}
	
	responses.Set("200", &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Description: func() *string { s := "Success"; return &s }(),
		},
	})
	
	paths.Set("/test", &openapi3.PathItem{
		Get: &openapi3.Operation{
			OperationID: "testOp",
			Responses:   responses,
		},
	})
	
	swagger.Paths = paths

	result := generator.requestResponseBuilders(swagger)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "GetTestResponseInterface")
}

func TestGenerator_builders_Simple(t *testing.T) {
	generator := setupResponsesTestSimple()

	swagger := &openapi3.T{}
	paths := &openapi3.Paths{}
	responses := &openapi3.Responses{}
	
	responses.Set("201", &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Description: func() *string { s := "Created"; return &s }(),
		},
	})
	
	paths.Set("/create", &openapi3.PathItem{
		Post: &openapi3.Operation{
			OperationID: "createOp",
			Responses:   responses,
		},
	})
	
	swagger.Paths = paths

	result := generator.builders(swagger)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "PostCreateStatus201Builder")
}

func TestGenerator_handlersTypes(t *testing.T) {
	generator := setupResponsesTestSimple()

	swagger := &openapi3.T{}
	paths := &openapi3.Paths{}
	responses := &openapi3.Responses{}
	
	responses.Set("200", &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Description: func() *string { s := "OK"; return &s }(),
		},
	})
	
	paths.Set("/handlers", &openapi3.PathItem{
		Get: &openapi3.Operation{
			OperationID: "handlersOp",
			Responses:   responses,
		},
	})
	
	swagger.Paths = paths

	result := generator.handlersTypes(swagger)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.NotEmpty(t, codeStr)
}

func TestGenerator_handlersInterfaces(t *testing.T) {
	generator := setupResponsesTestSimple()

	swagger := &openapi3.T{}
	paths := &openapi3.Paths{}
	responses := &openapi3.Responses{}
	
	responses.Set("200", &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Description: func() *string { s := "OK"; return &s }(),
		},
	})
	
	paths.Set("/interfaces", &openapi3.PathItem{
		Get: &openapi3.Operation{
			OperationID: "interfacesOp",
			Responses:   responses,
		},
	})
	
	swagger.Paths = paths

	result := generator.handlersInterfaces(swagger)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.NotEmpty(t, codeStr)
}

func TestGenerator_responseStruct(t *testing.T) {
	generator := setupResponsesTestSimple()

	result := generator.responseStruct()
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "Response")
}

func TestGenerator_responseInterface(t *testing.T) {
	generator := setupResponsesTestSimple()

	result := generator.responseInterface("TestResponse")
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "TestResponse")
}

func TestGenerator_responseType(t *testing.T) {
	generator := setupResponsesTestSimple()

	result := generator.responseType("TestResponseType")
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "TestResponseType")
}

func TestGenerator_responseImplementationFunc(t *testing.T) {
	generator := setupResponsesTestSimple()

	result := generator.responseImplementationFunc("TestImpl")
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "TestImpl")
}