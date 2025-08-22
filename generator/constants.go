package generator

// OpenAPI Type Constants
const (
	TypeString  = "string"
	TypeInteger = "integer"
	TypeNumber  = "number"
	TypeBoolean = "boolean"
	TypeObject  = "object"
	TypeArray   = "array"
)

// OpenAPI Format Constants
const (
	FormatByte                  = "byte"
	FormatBinary                = "binary"
	FormatEmail                 = "email"
	FormatDate                  = "date"
	FormatDateTime              = "date-time"
	FormatISO4217CurrencyCode   = "iso4217-currency-code"
	FormatISO3166Alpha2         = "iso3166-alpha-2"
	FormatISO3166Alpha3         = "iso3166-alpha-3"
	FormatUUID                  = "uuid"
	FormatJSON                  = "json"
)

// Parameter In Constants
const (
	InHeader = "header"
	InPath   = "path"
	InQuery  = "query"
	InBody   = "Body"
)

// Suffix Constants
const (
	SuffixRequestBody    = "RequestBody"
	SuffixRequest        = "Request"
	SuffixResponse       = "Response"
	SuffixEnum           = "Enum"
	SuffixRegex          = "Regex"
	SuffixMapEntry       = "MapEntry"
	SuffixSliceElement   = "SliceElement"
)

// Package Constants
const (
	PackageRegexp      = "regexp"
	PackageEncodingJSON = "encoding/json"
	PackageFmt         = "fmt"
	PackageStrings     = "strings"
	PackageNetHTTP     = "net/http"
)

// Standard Field Names
const (
	FieldProcessingResult      = "ProcessingResult"
	FieldSecurityCheckResults  = "SecurityCheckResults"
	FieldSecurityScheme        = "SecurityScheme"
	FieldContentType           = "contentType"
	FieldRedirectURL           = "redirectURL"
	FieldHeaders               = "headers"
	FieldValue                 = "Value"
)

// Standard Method Names
const (
	MethodUnmarshalJSON = "UnmarshalJSON"
	MethodValidate      = "Validate"
	MethodCheck         = "Check"
	MethodGet           = "Get"
	MethodMustCompile   = "MustCompile"
	MethodMatchString   = "MatchString"
	MethodTrimSpace     = "TrimSpace"
	MethodErrorf        = "Errorf"
	MethodUnmarshal     = "Unmarshal"
)

// Extension Name Constants (matching type.go constants)
const (
	ExtGoType              = "x-go-type"
	ExtGoMapType           = "x-go-map-type"
	ExtGoTypeStringParse   = "x-go-type-string-parse"
	ExtGoPointer           = "x-go-pointer"
	ExtGoRegex             = "x-go-regex"
	ExtGoStringTrimmable   = "x-go-string-trimmable"
	ExtGoOmitempty         = "x-go-omitempty"
	ExtGoSkipValidation    = "x-go-skip-validation"
	ExtGoSkipSecurityCheck = "x-go-skip-security-check"
)

// Error Message Templates
const (
	ErrorInvalidEnum          = "invalid %s enum value"
	ErrorFieldRequired        = "%s is required"
	ErrorRegexNotMatched      = "%s not matched by the '%s' regex"
	ErrorUnexpectedExtType    = "unexpected type for %s extension: %T"
)

// Error Context Keys
const (
	ContextSchemaType     = "schema_type"
	ContextFieldName      = "field_name"
	ContextExtensionName  = "extension_name"
	ContextReceivedType   = "received_type"
	ContextReceivedValue  = "received_value"
	ContextExpectedTypes  = "expected_types"
	ContextOperation      = "operation"
	ContextReason         = "reason"
)