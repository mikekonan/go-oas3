# Go OpenAPI v3 Server Code Generator

A powerful, modern code generator that creates clean server boilerplate from OpenAPI v3 specifications. Built with Go 1.24+ and the latest dependencies, featuring enhanced error handling and extensive customization options.

## Table of Contents

- [Quick Start](#quick-start)
- [Key Ideas](#key-ideas)
- [Installation](#installation)
- [Usage](#usage)
- [Complete Workflow Example](#complete-workflow-example)
- [OpenAPI Features](#openapi-features)
- [Extensions Reference](#extensions-reference)
- [Limitations](#limitations)
- [Contributing](#contributing)

## Quick Start

### Prerequisites
- **Go 1.24+** (updated for latest language features)
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

**Note:** Path stubs generation relies on the **first tag** from your paths. While tags are not required, they are **strongly recommended** for better organization:
- **With tags**: Creates separate service interfaces per tag (e.g., `UserService`, `OrderService`)
- **Without tags**: All operations are grouped under a single `DefaultService` interface

Example with tags:
```yaml
paths:
  /users:
    get:
      tags: [users]  # Creates UserService interface
  /orders:
    post:
      tags: [orders]  # Creates OrderService interface
```
## Usage

### Command Line Arguments

| Flag | Type | Description | Default |
|------|------|-------------|---------|
| `-swagger-addr` | string | Path or URL to OpenAPI specification | `swagger.yaml` |
| `-package` | string | **Required.** Go package name for generated code | - |
| `-path` | string | **Required.** Output directory for generated files | - |
| `-componentsPackage` | string | Package name for components (if different from main) | Same as `-package` |
| `-componentsPath` | string | Path for components (if different from main) | Same as `-path` |
| `-authorization` | string | Headers for remote swagger files (`key1:value1,key2:value2`) | - |
| `-prioritize-x-go-type` | bool | Prioritize `x-go-type` over schema properties | `false` |
| `-pass-raw-request` | bool | Pass raw HTTP request to handler functions | `false` |

### Examples

```bash
# Basic local generation
go-oas3 -swagger-addr swagger.yaml -package api -path ./generated

# Remote file with custom components
go-oas3 \
  -swagger-addr https://petstore.swagger.io/v2/swagger.json \
  -package petstore \
  -path ./api \
  -componentsPackage models \
  -componentsPath ./api/models

# With authorization headers
go-oas3 \
  -swagger-addr https://api.example.com/swagger.yaml \
  -package myapi \
  -path ./generated \
  -authorization "X-API-Key:secret,Authorization:Bearer token"
```

## Complete Workflow Example

Here's a complete example from OpenAPI spec to running server:

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
            x-go-type: github.com/google/uuid.UUID
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
go-oas3 -swagger-addr api.yaml -package userapi -path ./generated
```

### 3. Implement Handlers
```go
package main

import (
    "context"
    "net/http"
    "github.com/go-chi/chi/v5"
    "./generated"
)

type UsersService struct{}

func (s *UsersService) GetUsersId(ctx context.Context, request userapi.GetUsersIdRequestObject) userapi.GetUsersIdResponseObject {
    // Your business logic here
    user := userapi.User{
        Id:    request.Id,
        Email: "user@example.com",
        Name:  "John Doe",
    }
    
    return userapi.GetUsersId200JSONResponse{
        Body: user,
    }
}

func main() {
    service := &UsersService{}
    
    r := chi.NewRouter()
    userapi.HandlerFromMux(service, r)
    
    http.ListenAndServe(":8080", r)
}
```

### 4. Set Up Your Project
```bash
# Initialize Go module
go mod init myproject

# Add dependencies and run
go mod tidy
go run main.go
```

### 5. Test Your API
```bash
# Test the endpoint
curl http://localhost:8080/users/550e8400-e29b-41d4-a716-446655440000

# Response:
# {
#   "id": "550e8400-e29b-41d4-a716-446655440000",
#   "email": "user@example.com", 
#   "name": "John Doe"
# }
```

The generated boilerplate includes:
- **Type-safe handlers** - Request/response objects with proper Go types
- **Automatic validation** - Built-in validation for all parameters and request bodies
- **Router integration** - Ready-to-use chi router setup
- **Error handling** - Structured error responses matching your OpenAPI spec

## OpenAPI Features

### Required Fields
Path, query, component, and header required fields are supported.

### Security
Security schemas for HTTP and API key (header/cookie) are supported.

### Cookie
Response header `Set-Cookie` is supported. *Cookie in request is supported via security schema only.*

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

## Limitations

### Known Limitations

#### Inline Schema Responses
The generator currently has limited support for inline schemas in responses. For example:

```yaml
# ❌ Not fully supported - may cause compilation issues
responses:
  '200':
    content:
      application/json:
        schema:
          type: object
          properties:
            message: 
              type: string

# ✅ Recommended - use $ref to components
responses:
  '200':
    content:
      application/json:
        schema:
          $ref: '#/components/schemas/SuccessResponse'
```

**Workaround**: Define all response schemas in `components/schemas` and reference them using `$ref`.

#### Anonymous Types
- Anonymous slice elements and map entries have limited support
- Complex nested anonymous types may not generate correctly

**Workaround**: Define explicit component schemas for complex types.

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

