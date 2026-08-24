# Go OpenAPI v3 Server Code Generator

Generates Go server boilerplate from an OpenAPI v3 specification: a chi router, request parsing, validation, and response builders that only let a handler answer with what the spec declares.

## Table of Contents

- [Quick Start](#quick-start)
- [Key Ideas](#key-ideas)
- [Installation](#installation)
- [Usage](#usage)
- [Complete Workflow Example](#complete-workflow-example)
- [OpenAPI Features](#openapi-features)
- [Extensions Reference](#extensions-reference)
- [Generic Response Mode](#generic-response-mode)
- [Limitations](#limitations)
- [Questions or Feature Requests](#questions-or-feature-requests)
- [Contributing](#contributing)

## Quick Start

### Prerequisites
- **Go 1.22+**
- Valid OpenAPI 3.0+ specification file
- Basic understanding of Go modules

### Installation

```bash
go install github.com/mikekonan/go-oas3@latest
```

### Basic Usage

```bash
# Generate from local file
go-oas3 -swagger-addr swagger.yaml -package myapi -path ./generated

# Generate from remote URL
go-oas3 -swagger-addr https://example.com/api/swagger.yaml -package myapi -path ./generated
```

## Key Ideas

- **Request Parsing**: Stubs handle all request parsing logic automatically
- **Type Safety**: Response builders ensure you can only respond according to your specification  
- **Validation**: Built-in validation for all request parameters and bodies
- **Security**: Automatic security middleware generation from OpenAPI security schemes

**Note:** Path stub generation relies on the **first tag** from your paths. While tags are not required, they are **strongly recommended** for better organization:
- **With tags**: Creates separate service interfaces per tag (e.g., `UsersService`, `OrdersService`)
- **Without tags**: All operations are grouped under a single `DefaultService` interface

Example with tags:
```yaml
paths:
  /users:
    get:
      tags: [users]  # Creates UsersService interface
  /orders:
    post:
      tags: [orders]  # Creates OrdersService interface
```
## Usage

### Command Line Arguments

| Flag | Type | Description | Default |
|------|------|-------------|---------|
| `-swagger-addr` | string | Path or URL to OpenAPI specification | `swagger.yaml` |
| `-package` | string | **Required.** Import path of the generated router package (e.g. `myproject/api`) | - |
| `-path` | string | **Required.** Output directory for generated files | - |
| `-componentsPackage` | string | Import path of the components package — a full import path, not a bare name | Same as `-package` |
| `-componentsPath` | string | Path for components (if different from main) | Same as `-path` |
| `-authorization` | string | Headers for remote swagger files (`key1:value1,key2:value2`) | - |
| `-prioritize-x-go-type` | bool | Prioritize `x-go-type` over schema properties | `false` |
| `-pass-raw-request` | bool | Pass raw HTTP request to handler functions | `false` |
| `-default-generics` | bool | Generate every operation in generic mode by default (see [Generic Response Mode](#generic-response-mode)); can be overridden per-operation with `x-go-generics`. | `false` |
| `-version` | bool | Print version, Go version and build info, then exit | `false` |

**`-package` is an import path, not a package name.** The `package X` clause of the
generated files always follows the *output directory* name — `-path ./api/v1/gen`
produces `package gen`. `-package` / `-componentsPackage` are what the generated
code uses to `import` across packages, so they must be resolvable import paths
(`myproject/api/models`, not `models`) whenever `-componentsPath` differs from
`-path`.

### Examples

```bash
# Basic local generation
go-oas3 -swagger-addr swagger.yaml -package api -path ./generated

# Remote file with custom components
go-oas3 \
  -swagger-addr https://petstore3.swagger.io/api/v3/openapi.json \
  -package myproject/api \
  -path ./api \
  -componentsPackage myproject/api/models \
  -componentsPath ./api/models

# With authorization headers
go-oas3 \
  -swagger-addr https://api.example.com/swagger.yaml \
  -package myapi \
  -path ./generated \
  -authorization "X-API-Key:secret,Authorization:Bearer token"
```

## Complete Workflow Example

Here's a complete example from an OpenAPI spec to a running server:

### 1. Create OpenAPI Specification (`api.yaml`)
```yaml
openapi: 3.0.0
info:
  title: User API
  version: 1.0.0
paths:
  /users/{id}:
    get:
      tags: [users]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: User found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
        '404':
          description: User not found
components:
  schemas:
    User:
      type: object
      required: [id, email]
      properties:
        id:
          type: string
          format: uuid
        email:
          type: string
          format: email
        name:
          type: string
          x-go-omitempty: true
```

### 2. Generate Code
```bash
go mod init myproject
go-oas3 -swagger-addr api.yaml -package myproject/generated -path ./generated
```

The generated files land in `./generated` as `package generated` (the directory
name), and `spec_gen.go` embeds the spec itself.

### 3. Implement Handlers

The generator emits one `<Tag>Service` interface and one `<Tag>Handler`
constructor per first tag. Responses are built through a typed cascade, so a
handler can only answer with a status code and content type the spec declares.

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"

    userapi "myproject/generated"
)

type usersService struct{}

func (usersService) GetUsersID(ctx context.Context, request userapi.GetUsersIDRequest) userapi.GetUsersIDResponse {
    // Parsing and validation already happened — this reports the outcome.
    if err := request.ProcessingResult.Err(); err != nil {
        return userapi.GetUsersIDResponseBuilder().StatusCode404().Build()
    }

    return userapi.GetUsersIDResponseBuilder().
        StatusCode200().
        ApplicationJson().
        Body(userapi.User{
            ID:    request.Path.ID, // uuid.UUID, already parsed
            Email: "user@example.com",
            Name:  "John Doe",
        }).
        Build()
}

func main() {
    r := chi.NewRouter()

    // hooks (*userapi.Hooks) is optional — nil installs no-op stubs.
    userapi.UsersHandler(usersService{}, r, nil)

    log.Fatal(http.ListenAndServe(":8080", r))
}
```

Naming follows the spec: `GET /users/{id}` with `tags: [users]` becomes
`UsersService.GetUsersID`, taking `GetUsersIDRequest` and returning
`GetUsersIDResponse`. Path, query and header parameters arrive as
`request.Path`, `request.Query`, `request.Header`.

When the spec declares security schemes, `UsersHandler` takes one extra
argument — a `SecuritySchemas` implementation that validates each credential.

### 4. Set Up Your Project
```bash
# Pull in the libraries the generated code needs (chi, ozzo-validation, uuid, ...)
go mod tidy

go run .
```

### 5. Test Your API
```bash
# Test the endpoint
curl http://localhost:8080/users/550e8400-e29b-41d4-a716-446655440000

# Response:
# {"email":"user@example.com","id":"550e8400-e29b-41d4-a716-446655440000","name":"John Doe"}
```

The generated boilerplate includes:
- **Type-safe handlers** - Request/response objects with proper Go types
- **Automatic validation** - Built-in validation for all parameters and request bodies
- **Router integration** - Ready-to-use chi router setup
- **Error handling** - Structured error responses matching your OpenAPI spec

## OpenAPI Features

### Required Fields
Required fields in path, query, headers, and components are supported.

### Security
Security schemes for HTTP and API key (header/cookie) are supported.

### Cookie
Response header `Set-Cookie` is supported. Cookies in requests are supported via security schemes only.

### Validation
Type validation supports the following data types:
- **string**: minLength, maxLength
- **number**, **integer**: minimum, maximum, exclusiveMinimum, exclusiveMaximum

### Custom Types
The generator supports several OpenAPI types for components:

| OpenAPI Type | Go Type |
|---|---|
| uuid | [github.com/google/uuid.UUID](https://github.com/google/uuid) |
| iso4217-currency-code | [github.com/mikekonan/go-types/v2/currency.Code](https://github.com/mikekonan/go-types) |
| iso3166-alpha-2 | [github.com/mikekonan/go-types/v2/country.Alpha2Code](https://github.com/mikekonan/go-types) |
| iso3166-alpha-3 | [github.com/mikekonan/go-types/v2/country.Alpha3Code](https://github.com/mikekonan/go-types) |

## Extensions Reference

The generator supports powerful OpenAPI extensions to customize Go code generation:

### Core Type Extensions

#### `x-go-type` - Custom Go Types
Specify custom Go types for any schema:

```yaml
# Use encoding/json.RawMessage for flexible JSON
metadata:
  type: object
  x-go-type: encoding/json.RawMessage

# Use third-party types
user_id:
  type: string
  x-go-type: github.com/google/uuid.UUID

# Use custom domain types
amount:
  type: string
  x-go-type: github.com/shopspring/decimal.Decimal
```

On a **path, query or header parameter** of `type: string`, `x-go-type` alone is
not enough — the generated code would assign a `string` to your type and fail to
compile. Pair it with [`x-go-type-string-parse`](#x-go-type-string-parse---custom-parsing),
or drop `x-go-type` and use a known `format` (see [Custom Types](#custom-types)).

#### `x-go-type-string-parse` - Custom Parsing
For string parameters that need custom parsing:

```yaml
# Custom UUID parsing from string
user_id:
  type: string
  x-go-type: github.com/google/uuid.UUID
  x-go-type-string-parse: github.com/google/uuid.Parse

# Custom time parsing
created_at:
  type: string
  x-go-type: time.Time
  x-go-type-string-parse: github.com/spf13/cast.ToTimeE
```

### Field Modifiers

#### `x-go-pointer` - Force Pointer Types
```yaml
# Make field a pointer (useful for optional fields)
optional_amount:
  type: integer
  x-go-pointer: true
  # Generates: OptionalAmount *int `json:"optional_amount,omitempty"`
```

#### `x-go-omitempty` - JSON Omitempty Tag
```yaml
# Add omitempty to JSON tag
description:
  type: string
  x-go-omitempty: true
  # Generates: Description string `json:"description,omitempty"`
```

#### `x-go-string-trimmable` - Auto-trim Strings
```yaml
# Automatically trim whitespace before validation
title:
  type: string
  minLength: 1
  x-go-string-trimmable: true
  # Trims spaces before checking minLength
```

### Validation Extensions

#### `x-go-regex` - Regex Validation
```yaml
# Add regex validation to parameters
phone:
  type: string
  x-go-regex: ^\+?[1-9]\d{1,14}$
  # Generates validation code with regexp.MustCompile
```

#### `x-go-skip-validation` - Disable Validation
```yaml
# Skip validation for performance-critical paths
large_payload:
  type: object
  properties:
    data: 
      type: string
  x-go-skip-validation: true
```

### Generic Mode Extension

#### `x-go-generics` - Per-Operation Generic Response Mode
Opt an operation into generic-mode generation. The response builder cascade
(20+ types per operation) is replaced by a single shared `*Response[B]`. See
[Generic Response Mode](#generic-response-mode) for the full picture.

```yaml
paths:
  /widgets:
    post:
      x-go-generics: true
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Widget'
```

Use `-default-generics` on the CLI to flip the default for the whole spec; then
`x-go-generics: false` can opt individual operations back to classic.

### Security Extensions

#### `x-go-skip-security-check` - Skip Security Validation
```yaml
# For operations that parse auth but don't enforce it
paths:
  /health:
    get:
      x-go-skip-security-check: true
      security:
        - ApiKeyAuth: []
      # Parses auth header but doesn't fail on missing/invalid auth
```

### Advanced Map Types

#### `x-go-map-type` - Custom Map Types
```yaml
# Custom map with typed keys/values
metadata:
  type: object
  additionalProperties:
    type: string
  x-go-map-type: map[github.com/google/uuid.UUID]string
  # Generates: map[uuid.UUID]string instead of map[string]string
```

### Complete Extension Example

```yaml
components:
  schemas:
    User:
      type: object
      required: [id, email]
      properties:
        id:
          type: string
          x-go-type: github.com/google/uuid.UUID
        email:
          type: string
          format: email
          x-go-regex: ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$
        profile:
          type: object
          x-go-pointer: true
          x-go-omitempty: true
          properties:
            bio:
              type: string
              x-go-string-trimmable: true
        metadata:
          type: object
          additionalProperties:
            type: string
          x-go-type: map[string]interface{}
      x-go-skip-validation: false

  parameters:
    UserID:
      name: user_id
      in: path
      required: true
      schema:
        type: string
        x-go-type: github.com/google/uuid.UUID
        x-go-type-string-parse: github.com/google/uuid.Parse
```

## Generic Response Mode

By default the generator emits a dedicated response-builder cascade for every
operation: `XxxResponseBuilder`, `XxxStatusCodeBuilder`, `Xxx200ContentTypeBuilder`,
`Xxx200ApplicationJsonResponseBuilder`, plus their methods. For a spec with
several status codes per endpoint that's ~20 types and ~280 lines of generated
code *per operation*.

**Generic mode** collapses this into a single shared `*Response[B]` type per
package. Opt in by marking the operation with `x-go-generics: true` (or flip
the global default via `-default-generics`).

### What changes

- The service-interface return type becomes `*Response[BodyT]` where `BodyT`
  is inferred from the 2xx response's `application/json` schema (`any` if no
  body is declared).
- The per-operation builder cascade is **not** emitted.
- The request struct embeds `RequestMeta` (shared `ProcessingResult` +
  `SecurityCheckResults`) and gains a `GetHeader() any` accessor, so it
  satisfies the package-level `Request` interface.

### Generated helpers in every package

When at least one operation uses `x-go-generics`, the generator emits these
one-time symbols:

```go
// Implemented by every generic request — application helpers can accept any
// generic request as a single parameter.
type Request interface {
    GetProcessingResult() RequestProcessingResult
    GetSecurityCheckResults() map[SecurityScheme]string
    GetHeader() any
}

type RequestMeta struct {
    ProcessingResult     RequestProcessingResult
    SecurityCheckResults map[SecurityScheme]string
}

// Single shared response type — replaces the per-operation cascade.
type Response[B any] struct { /* ... */ }
type ResponseBuilder[B any] struct { /* ... */ }

func NewResponse[B any]() *ResponseBuilder[B]

// Fluent setters on ResponseBuilder[B]:
//   .Status(code int)
//   .ContentType(ct string)          // default "application/json"
//   .ApplicationJson()
//   .ApplicationXml()
//   .ApplicationOctetStream()
//   .TextHtml() / .TextPlain() / .TextCsv()
//   .Body(B)                          // typed success body
//   .BodyAny(any)                     // error body of a different type
//   .BodyRaw([]byte)
//   .Header(key, value string)
//   .Cookie(c http.Cookie)
//   .Redirect(url string)
//   .Build() *Response[B]
```

### Other shared helpers (emitted for all modes)

These were extracted to keep per-operation code small regardless of generic
mode:

- `respond(w, r, hooks, name, response, hasContent)` — single shared HTTP
  writer. Per-operation handlers become 4 lines.
- `ensureHooks(*Hooks) *Hooks` — fills every nil hook with a no-op stub, so
  generated code calls hooks unconditionally.
- `extractSecurity(r, hooks, name, requirements, skipCheck)` — runs the
  OR-of-AND security matrix; replaces ~80 inlined lines per operation.

### Example

```yaml
paths:
  /widgets:
    post:
      tags: [widgets]
      x-go-generics: true
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateWidgetRequest'
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Widget'
        '400':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
```

Service implementation:

```go
func (s *widgetsService) PostWidgets(ctx context.Context, req PostWidgetsRequest) *Response[Widget] {
    if err := req.ProcessingResult.Err(); err != nil {
        return NewResponse[Widget]().Status(400).
            BodyAny(ErrorBody{Code: "bad_request", Message: err.Error()}).
            Build()
    }
    return NewResponse[Widget]().Status(200).
        Body(Widget{ID: "w-1", Name: req.Body.Name}).
        Build()
}
```

### Coexistence with classic mode

Generic and classic operations live side-by-side in the same generated
package — only operations marked with `x-go-generics: true` (or covered by
`-default-generics`) use the new shape. Migration can be done one operation
at a time without touching the rest of the codebase.

## Limitations

### Known Limitations

#### Nested Inline Schemas
A *flat* inline response schema works — it becomes a named type such as
`GetPing200ApplicationJson`:

```yaml
# ✅ Works
responses:
  '200':
    content:
      application/json:
        schema:
          type: object
          properties:
            message:
              type: string
```

Nesting an anonymous object one level deeper does not. Both of these break
generation: `components_gen.go` is written empty and the output no longer compiles.

```yaml
# ❌ Nested inline object
schema:
  type: object
  properties:
    inner:
      type: object
      properties:
        deep: { type: string }

# ❌ Inline array items
schema:
  type: object
  properties:
    list:
      type: array
      items:
        type: object
        properties:
          name: { type: string }
```

**Workaround**: give every nested object its own entry in `components/schemas`
and reference it with `$ref`.

```yaml
# ✅ Recommended
responses:
  '200':
    content:
      application/json:
        schema:
          $ref: '#/components/schemas/SuccessResponse'
```

#### Specs Without `components`
A spec with no `components:` section at all makes the generator panic. Declare at
least one component schema.

### Best Practices

1. **Always use $ref for schemas** - Avoid inline type definitions
2. **Define reusable components** - Create schemas in `components/schemas`
3. **Use meaningful names** - Component names become Go type names
4. **Test generated code** - Always compile and test after generation


## Questions or Feature Requests

Have a question or need some functionality? Feel free to open an issue or submit a pull request.

## Contributing

Go OpenAPI v3 server code generator uses [github.com/dave/jennifer](https://github.com/dave/jennifer) for code generation. 
Using [github.com/aloder/tojen](https://github.com/aloder/tojen) is the suggested way to generate Jennifer code.

