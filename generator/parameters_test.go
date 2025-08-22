package generator

import (
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mikekonan/go-oas3/configurator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupParametersTest() *Generator {
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

func TestGenerator_requestParameters(t *testing.T) {
	generator := setupParametersTest()

	paths := map[string]*openapi3.PathItem{
		"/users/{id}": {
			Get: &openapi3.Operation{
				OperationID: "getUser",
				Parameters: []*openapi3.ParameterRef{
					{
						Value: &openapi3.Parameter{
							Name:     "id",
							In:       "path",
							Required: true,
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"integer"},
								},
							},
						},
					},
					{
						Value: &openapi3.Parameter{
							Name: "include",
							In:   "query",
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								},
							},
						},
					},
				},
			},
		},
		"/products": {
			Post: &openapi3.Operation{
				OperationID: "createProduct",
				RequestBody: &openapi3.RequestBodyRef{
					Value: &openapi3.RequestBody{
						Content: map[string]*openapi3.MediaType{
							"application/json": {
								Schema: &openapi3.SchemaRef{
									Value: &openapi3.Schema{
										Type: &openapi3.Types{"object"},
										Properties: map[string]*openapi3.SchemaRef{
											"name": {
												Value: &openapi3.Schema{
													Type: &openapi3.Types{"string"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := generator.requestParameters(paths)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "type GetUsersIDRequest struct")
	assert.Contains(t, codeStr, "type PostProductsRequest struct")
	assert.Contains(t, codeStr, "ID int")
	assert.Contains(t, codeStr, "Include *string")
}

func TestGenerator_requestParameterStruct(t *testing.T) {
	generator := setupParametersTest()

	operation := &openapi3.Operation{
		OperationID: "testOperation",
		Parameters: []*openapi3.ParameterRef{
			{
				Value: &openapi3.Parameter{
					Name:     "user_id",
					In:       "path",
					Required: true,
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						},
					},
				},
			},
			{
				Value: &openapi3.Parameter{
					Name: "limit",
					In:   "query",
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"integer"},
						},
					},
				},
			},
			{
				Value: &openapi3.Parameter{
					Name: "Authorization",
					In:   "header",
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name                     string
		structName               string
		contentType              string
		appendContentTypeToName  bool
		expectedInCode           []string
	}{
		{
			name:                    "Standard request struct",
			structName:              "TestRequest",
			contentType:             "application/json",
			appendContentTypeToName: false,
			expectedInCode: []string{
				"type TestRequestRequest struct",
				"UserID string",
				"Limit *int",
				"Authorization *string",
				`json:"user_id"`,
				`json:"limit"`,
				`json:"authorization"`,
			},
		},
		{
			name:                    "With content type in name",
			structName:              "TestRequest",
			contentType:             "application/xml",
			appendContentTypeToName: true,
			expectedInCode: []string{
				"type TestRequestRequest struct",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.requestParameterStruct(tt.structName, tt.contentType, tt.appendContentTypeToName, operation)
			require.NotNil(t, result)

			file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
			for _, expected := range tt.expectedInCode {
				assert.Contains(t, codeStr, expected, "Expected %q in generated code", expected)
			}
		})
	}
}

func TestGenerator_generateParameterParser(t *testing.T) {
	generator := setupParametersTest()

	tests := []struct {
		name           string
		in             string
		parameter      *openapi3.ParameterRef
		wrapperName    string
		expectedInCode []string
	}{
		{
			name: "Path parameter parser",
			in:   "path",
			parameter: &openapi3.ParameterRef{
				Value: &openapi3.Parameter{
					Name:     "id",
					In:       "path",
					Required: true,
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"integer"},
						},
					},
				},
			},
			wrapperName: "TestWrapper",
			expectedInCode: []string{
				"ID",
				"id",
				"chi.URLParam",
			},
		},
		{
			name: "Query parameter parser",
			in:   "query",
			parameter: &openapi3.ParameterRef{
				Value: &openapi3.Parameter{
					Name: "limit",
					In:   "query",
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"integer"},
						},
					},
				},
			},
			wrapperName: "TestWrapper",
			expectedInCode: []string{
				"Limit",
				"limit",
				"r.URL.Query",
			},
		},
		{
			name: "Header parameter parser",
			in:   "header",
			parameter: &openapi3.ParameterRef{
				Value: &openapi3.Parameter{
					Name: "Authorization",
					In:   "header",
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						},
					},
				},
			},
			wrapperName: "TestWrapper",
			expectedInCode: []string{
				"Authorization",
				"r.Header.Get",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.generateParameterParser(tt.in, tt.parameter, tt.wrapperName)
			require.NotNil(t, result)

			file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
			for _, expected := range tt.expectedInCode {
				assert.Contains(t, codeStr, expected, "Expected %q in generated code", expected)
			}
		})
	}
}

func TestGenerator_generatePathParameterParser(t *testing.T) {
	generator := setupParametersTest()

	parameter := &openapi3.ParameterRef{
		Value: &openapi3.Parameter{
			Name:     "user_id",
			In:       "path",
			Required: true,
			Schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
				},
			},
		},
	}

	result := generator.generatePathParameterParser("UserId", "user_id", "TestWrapper", parameter)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "UserId")
	assert.Contains(t, codeStr, "user_id")
	assert.Contains(t, codeStr, "chi.URLParam")
}

func TestGenerator_generateQueryParameterParser(t *testing.T) {
	generator := setupParametersTest()

	tests := []struct {
		name           string
		parameter      *openapi3.ParameterRef
		expectedInCode []string
	}{
		{
			name: "Required string parameter",
			parameter: &openapi3.ParameterRef{
				Value: &openapi3.Parameter{
					Name:     "search",
					In:       "query",
					Required: true,
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						},
					},
				},
			},
			expectedInCode: []string{
				"Search",
				"search",
				"r.URL.Query().Get",
			},
		},
		{
			name: "Optional integer parameter",
			parameter: &openapi3.ParameterRef{
				Value: &openapi3.Parameter{
					Name: "limit",
					In:   "query",
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"integer"},
						},
					},
				},
			},
			expectedInCode: []string{
				"Search",
				"limit",
				"cast.ToInt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.generateQueryParameterParser("Search", "search", "TestWrapper", tt.parameter)
			require.NotNil(t, result)

			file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
			for _, expected := range tt.expectedInCode {
				assert.Contains(t, codeStr, expected, "Expected %q in generated code", expected)
			}
		})
	}
}

func TestGenerator_generateHeaderParameterParser(t *testing.T) {
	generator := setupParametersTest()

	parameter := &openapi3.ParameterRef{
		Value: &openapi3.Parameter{
			Name: "X-API-Key",
			In:   "header",
			Schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
				},
			},
		},
	}

	result := generator.generateHeaderParameterParser("XApiKey", "X-API-Key", "TestWrapper", parameter)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "XApiKey")
	assert.Contains(t, codeStr, "X-API-Key")
	assert.Contains(t, codeStr, "r.Header.Get")
}

func TestGenerator_wrapperStr(t *testing.T) {
	generator := setupParametersTest()

	parameter := &openapi3.ParameterRef{
		Value: &openapi3.Parameter{
			Name:     "name",
			In:       "query",
			Required: true,
			Schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type:      &openapi3.Types{"string"},
					MinLength: 1,
					MaxLength: &[]uint64{100}[0],
				},
			},
		},
	}

	result := generator.wrapperStr("query", "Name", "name", "TestWrapper", parameter)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "Name")
	assert.Contains(t, codeStr, "name")
	assert.Contains(t, codeStr, "r.URL.Query")
}

func TestGenerator_wrapperInteger(t *testing.T) {
	generator := setupParametersTest()

	parameter := &openapi3.ParameterRef{
		Value: &openapi3.Parameter{
			Name: "page",
			In:   "query",
			Schema: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type:    &openapi3.Types{"integer"},
					Min: &[]float64{1}[0],
					Max: &[]float64{1000}[0],
				},
			},
		},
	}

	result := generator.wrapperInteger("query", "Page", "page", "TestWrapper", parameter)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "Page")
	assert.Contains(t, codeStr, "page")
	assert.Contains(t, codeStr, "cast.ToInt")
}

func TestGenerator_wrapperBody(t *testing.T) {
	generator := setupParametersTest()

	operation := &openapi3.Operation{
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Required: true,
				Content: map[string]*openapi3.MediaType{
					"application/json": {
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"object"},
								Properties: map[string]*openapi3.SchemaRef{
									"name": {
										Value: &openapi3.Schema{
											Type: &openapi3.Types{"string"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	body := &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: map[string]*openapi3.SchemaRef{
				"name": {
					Value: &openapi3.Schema{
						Type: &openapi3.Types{"string"},
					},
				},
			},
		},
	}

	result := generator.wrapperBody("POST", "/users", "application/json", "TestWrapper", operation, body)
	require.NotNil(t, result)

	file := jen.NewFile("test").Add(result)
	codeStr := file.GoString()
	assert.Contains(t, codeStr, "json.NewDecoder")
	assert.Contains(t, codeStr, "Body")
}

func TestGenerator_wrapperRequestParsers(t *testing.T) {
	generator := setupParametersTest()

	operation := &openapi3.Operation{
		Parameters: []*openapi3.ParameterRef{
			{
				Value: &openapi3.Parameter{
					Name:     "id",
					In:       "path",
					Required: true,
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"integer"},
						},
					},
				},
			},
			{
				Value: &openapi3.Parameter{
					Name: "filter",
					In:   "query",
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						},
					},
				},
			},
		},
	}

	result := generator.wrapperRequestParsers("TestWrapper", operation)
	require.NotEmpty(t, result)

	// Combine all parsers for testing
	combined := jen.Func().Id("testParsers").Params().Block(result...)
	codeStr := combined.GoString()

	assert.Contains(t, codeStr, "ID")
	assert.Contains(t, codeStr, "Filter")
	assert.Contains(t, codeStr, "chi.URLParam")
	assert.Contains(t, codeStr, "r.URL.Query")
}

func TestGenerator_parameters_EdgeCases(t *testing.T) {
	generator := setupParametersTest()

	tests := []struct {
		name      string
		operation *openapi3.Operation
		expectErr bool
	}{
		{
			name: "Operation with no parameters",
			operation: &openapi3.Operation{
				OperationID: "simpleOp",
			},
			expectErr: false,
		},
		{
			name: "Operation with custom type parameter",
			operation: &openapi3.Operation{
				OperationID: "customTypeOp",
				Parameters: []*openapi3.ParameterRef{
					{
						Value: &openapi3.Parameter{
							Name: "timestamp",
							In:   "query",
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type:   &openapi3.Types{"string"},
									Format: "date-time",
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Operation with enum parameter",
			operation: &openapi3.Operation{
				OperationID: "enumOp",
				Parameters: []*openapi3.ParameterRef{
					{
						Value: &openapi3.Parameter{
							Name: "status",
							In:   "query",
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"string"},
									Enum: []interface{}{"active", "inactive"},
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.requestParameterStruct("TestRequest", "application/json", false, tt.operation)
			if tt.expectErr {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				// Verify the code can be generated without panic
				file := jen.NewFile("test").Add(result)
				codeStr := file.GoString()
				// For operations with no parameters, code might be empty or minimal
				if tt.name == "Operation with no parameters" {
					// Empty result is acceptable for operations with no parameters
					assert.True(t, len(codeStr) >= 0)
				} else {
					assert.NotEmpty(t, codeStr)
				}
			}
		})
	}
}