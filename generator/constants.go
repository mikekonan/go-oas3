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
	InBody   = "body"  // Aligned with OAS3 terminology
	InCookie = "cookie" // Added for completeness
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
	ContextIssue          = "issue"
	ContextErrorMessage   = "error_message"
	ContextSchemaPrefix   = "schema_"
)

// Request Processing Constants
const (
	RequestProcessingResult         = "RequestProcessingResult"
	RequestBodyUnmarshalFailed      = "RequestBodyUnmarshalFailed"
	RequestHeaderParseFailed        = "RequestHeaderParseFailed"
	RequestQueryParseFailed         = "RequestQueryParseFailed"
	RequestPathParseFailed          = "RequestPathParseFailed"
	BodyUnmarshalFailed            = "BodyUnmarshalFailed"
	HeaderParseFailed              = "HeaderParseFailed"
	QueryParseFailed               = "QueryParseFailed"
	PathParseFailed                = "PathParseFailed"
)

// HTTP Constants
const (
	HTTPHeader = "Header"
	HTTPGet    = "Get"
	HTTPURL    = "URL"
	HTTPQuery  = "Query"
	HTTPBody   = "Body"
)

// Parameter Processing Constants
const (
	ParamStr      = "Str"
	ParamDefault  = "default"
	ParamRequired = "required"
	ParamError    = "error"
	ParamType     = "typee"  // "type" is reserved keyword, use typee
	ParamNil      = "nil"
	ParamIntVal   = "intVal"
	ParamDecodeErr = "decodeErr"
	ParamReadErr   = "readErr"
	ParamOk        = "ok"
	ParamBuf       = "buf"
)

// Field and Variable Names
const (
	VarRequest       = "request"
	VarRouter        = "router"
	VarResponse      = "response"
	VarHeaders       = "headers"
	VarBody          = "body"
	VarErr           = "err"
	VarR             = "r"
	VarHooks         = "hooks"
	VarChi           = "chi"
	VarUUID          = "uuid"
	VarUser          = "user"
	VarName          = "Name"
	VarID            = "ID"
	VarUUIDSuffix    = "uuid"
	VarIDSuffix      = "id"
)

// Normalization Constants
const (
	NormUUIDSuffix = "uuid"
	NormUUID       = "UUID"
	NormIDSuffix   = "id"
	NormID         = "ID"
)

// Special Values
const (
	ValueUnknown      = "unknown"
	ValueTest         = "test"
	ValueComponents   = "components"
	ValueAPI          = "api"
	ValueString       = "string"
	ValueInteger      = "integer"
	ValueNumber       = "number"
	ValueArray        = "array"
	ValueActive       = "active"
	ValueInactive     = "inactive"
	ValuePending      = "pending"
	ValueTrue         = "true"
)

// URL and Network Constants
const (
	URLParam      = "URLParam"
	URLQueryGet   = "Query().Get"
	HeaderGet     = "Header.Get"
	BodyIsNotByte = "body is not []byte"
)

// Package Imports (for generated code)
const (
	PackageIO         = "io/ioutil"
	PackageErrors     = "errors"
	PackageXML        = "encoding/xml"
	PackageUUID       = "github.com/google/uuid"
	PackageCast       = "github.com/spf13/cast"
	PackageCurrency   = "github.com/mikekonan/go-types/v2/currency"
	PackageCountry    = "github.com/mikekonan/go-types/v2/country"
	PackageOzzo       = "github.com/go-ozzo/ozzo-validation/v4"
)

// Method Names (for generated code)
const (
	MethodParse          = "Parse"
	MethodByCodeStrErr   = "ByCodeStrErr"
	MethodByAlpha2CodeStrErr = "ByAlpha2CodeStrErr"
	MethodCode           = "Code"
	MethodAlpha2Code     = "Alpha2Code"
	MethodToInt          = "ToInt"
	MethodToIntE         = "ToIntE"
	MethodReadAll        = "ReadAll"
	MethodNewDecoder     = "NewDecoder"
	MethodDecode         = "Decode"
	MethodNew            = "New"
	MethodAssert         = "Assert"
	MethodCall           = "Call"
	MethodDot            = "Dot"
)

// Validation Constants
const (
	ValidationField      = "Field"
	ValidationRequired   = "Required"
	ValidationRuneLength = "RuneLength"
	ValidationMin        = "Min"
	ValidationMax        = "Max"
	ValidationMatch      = "Match"
	ValidationExclusive  = "Exclusive"
	ValidationValidateStruct = "ValidateStruct"
)