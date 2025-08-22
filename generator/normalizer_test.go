package generator

import (
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizer_normalize(t *testing.T) {
	normalizer := &Normalizer{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple string",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "Camel case",
			input:    "helloWorld",
			expected: "HelloWorld",
		},
		{
			name:     "Snake case",
			input:    "hello_world",
			expected: "HelloWorld",
		},
		{
			name:     "Kebab case",
			input:    "hello-world",
			expected: "HelloWorld",
		},
		{
			name:     "With spaces",
			input:    "hello world",
			expected: "HelloWorld",
		},
		{
			name:     "With numbers",
			input:    "hello123world",
			expected: "Hello123world",
		},
		{
			name:     "UUID suffix",
			input:    "user_uuid",
			expected: "UserUUID",
		},
		{
			name:     "ID suffix",
			input:    "user_id",
			expected: "UserID",
		},
		{
			name:     "Complex separators",
			input:    "user-name@domain.com",
			expected: "UserNameDomainCom",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Single character",
			input:    "a",
			expected: "A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.normalize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizer_normalizeOperationName(t *testing.T) {
	normalizer := &Normalizer{}

	tests := []struct {
		name     string
		path     string
		method   string
		expected string
	}{
		{
			name:     "Simple GET",
			path:     "/users",
			method:   "GET",
			expected: "GetUsers",
		},
		{
			name:     "POST with path param",
			path:     "/users/{id}",
			method:   "POST",
			expected: "PostUsersID",
		},
		{
			name:     "Complex path",
			path:     "/api/v1/users/{userId}/posts",
			method:   "GET",
			expected: "GetApiV1UsersUserIdPosts",
		},
		{
			name:     "Root path",
			path:     "/",
			method:   "GET",
			expected: "Get",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.normalizeOperationName(tt.path, tt.method)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizer_extractNameFromRef(t *testing.T) {
	normalizer := &Normalizer{}

	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{
			name:     "Component schema ref",
			ref:      "#/components/schemas/User",
			expected: "User",
		},
		{
			name:     "Nested ref",
			ref:      "#/components/schemas/api/v1/UserProfile",
			expected: "UserProfile",
		},
		{
			name:     "Complex name",
			ref:      "#/components/schemas/user_profile_request",
			expected: "UserProfileRequest",
		},
		{
			name:     "Empty ref",
			ref:      "",
			expected: "",
		},
		{
			name:     "No slash ref",
			ref:      "User",
			expected: "User",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.extractNameFromRef(tt.ref)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizer_contentType(t *testing.T) {
	normalizer := &Normalizer{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Application JSON",
			input:    "application/json",
			expected: "ApplicationJson",
		},
		{
			name:     "Text plain",
			input:    "text/plain",
			expected: "TextPlain",
		},
		{
			name:     "Multipart form data",
			input:    "multipart/form-data",
			expected: "MultipartFormData",
		},
		{
			name:     "Application XML",
			input:    "application/xml",
			expected: "ApplicationXml",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.contentType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizer_decapitalize(t *testing.T) {
	normalizer := &Normalizer{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Capitalized word",
			input:    "Hello",
			expected: "hello",
		},
		{
			name:     "Already lowercase",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "Single character",
			input:    "A",
			expected: "a",
		},
		{
			name:     "Multiple words",
			input:    "HelloWorld",
			expected: "helloWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.decapitalize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizer_doubleLineAfterEachElement(t *testing.T) {
	normalizer := &Normalizer{}

	code1 := jen.Id("test1")
	code2 := jen.Id("test2")
	nullCode := jen.Null()
	lineCode := jen.Line()

	tests := []struct {
		name     string
		input    []jen.Code
		expected int // Expected number of elements (accounting for lines)
	}{
		{
			name:     "Two elements",
			input:    []jen.Code{code1, code2},
			expected: 6, // code1 + line + line + code2 + line + line
		},
		{
			name:     "Single element",
			input:    []jen.Code{code1},
			expected: 3, // code1 + line + line
		},
		{
			name:     "With null and line codes",
			input:    []jen.Code{code1, nullCode, code2, lineCode},
			expected: 6, // Only code1 and code2 should be processed
		},
		{
			name:     "Empty input",
			input:    []jen.Code{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.doubleLineAfterEachElement(tt.input...)
			assert.Len(t, result, tt.expected)
		})
	}
}

func TestNormalizer_lineAfterEachElement(t *testing.T) {
	normalizer := &Normalizer{}

	code1 := jen.Id("test1")
	code2 := jen.Id("test2")
	nullCode := jen.Null()
	lineCode := jen.Line()

	tests := []struct {
		name     string
		input    []jen.Code
		expected int // Expected number of elements (accounting for lines)
	}{
		{
			name:     "Two elements",
			input:    []jen.Code{code1, code2},
			expected: 4, // code1 + line + code2 + line
		},
		{
			name:     "Single element",
			input:    []jen.Code{code1},
			expected: 2, // code1 + line
		},
		{
			name:     "With null and line codes",
			input:    []jen.Code{code1, nullCode, code2, lineCode},
			expected: 4, // Only code1 and code2 should be processed
		},
		{
			name:     "Empty input",
			input:    []jen.Code{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.lineAfterEachElement(tt.input...)
			assert.Len(t, result, tt.expected)
		})
	}
}

// Benchmark tests for performance-critical normalization functions
func BenchmarkNormalizer_normalize(b *testing.B) {
	normalizer := &Normalizer{}
	testString := "user_profile_request_with_many_underscores_and_segments"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		normalizer.normalize(testString)
	}
}

func BenchmarkNormalizer_normalizeOperationName(b *testing.B) {
	normalizer := &Normalizer{}
	path := "/api/v1/users/{userId}/profiles/{profileId}/settings"
	method := "GET"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		normalizer.normalizeOperationName(path, method)
	}
}

func BenchmarkNormalizer_extractNameFromRef(b *testing.B) {
	normalizer := &Normalizer{}
	ref := "#/components/schemas/complex_user_profile_request_with_long_name"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		normalizer.extractNameFromRef(ref)
	}
}

// Helper function to test with edge cases
func TestNormalizer_EdgeCases(t *testing.T) {
	normalizer := &Normalizer{}

	t.Run("Unicode characters", func(t *testing.T) {
		result := normalizer.normalize("user-naïve")
		require.NotEmpty(t, result)
		// Should handle unicode gracefully
	})

	t.Run("Very long string", func(t *testing.T) {
		longString := "this_is_a_very_long_string_with_many_underscores_and_separators_that_should_be_normalized_properly"
		result := normalizer.normalize(longString)
		require.NotEmpty(t, result)
		assert.NotContains(t, result, "_")
	})

	t.Run("Only separators", func(t *testing.T) {
		result := normalizer.normalize("___---...")
		// Should handle gracefully
		require.NotNil(t, result)
	})
}