# Go OpenAPI v3 Server Code Generator

The purpose of this project is to generate clean server boilerplate code from OpenAPI v3 specifications. The generated code is based on the [github.com/go-chi/chi/v5](https://github.com/go-chi/chi) router. The generator processes all paths and components to generate Go structs and stubs. 

## Key Ideas

- Stubs take over the logic of parsing the request.
- Response builders encapsulate logic that doesn't allow you to respond differently from your specification.

**Note:** Path stubs generation relies on the **first tag** from your paths.

## Installation

```bash
go install github.com/mikekonan/go-oas3@latest
```
## Program Arguments

```text
Usage of go-oas3:
  -componentsPackage string
  -componentsPath string
  -package string
  -path string
  -swagger-addr string
    	 (default "swagger.yaml")
  -authorization string 
    a list of comma-separated key:value pairs to be sent as headers alongside each http request
  -prioritize-x-go-type
    by default, if both properties and x-go-type is provided, the generator will use properties.
    this flag will make generator prioritize x-go-type over properties.
  -pass-raw-request
    pass raw request to handler function
```
## Example

Run with:
```bash
go-oas3 -swagger-addr https://raw.githubusercontent.com/mikekonan/go-oas3/v1.0.62/example/swagger.yaml -package example -path ./example
```

The generated boilerplate and its client can be found in the [./example](./example) directory.

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

### Extensions

Specify a Go type with:
```yaml
    Component:
      properties:
        metadata:
          type: object
          x-go-type: encoding/json.RawMessage
```

Force pointer usage for the field:
```yaml
    Component:
      properties:
        amount:
          type: int
          x-go-pointer: true
```

Specify a regex to match a string:
```yaml
    Parameter:
      description: Parameter
      in: header
      name: parameter name
      required: true
      schema:
        type: string
        x-go-regex: ^[.?\d]+$
```

If you want to use your specific type (it has to declare function `Parse{TYPENAME} ({TYPENAME}, error)`) in query/path/header params:
```yaml
    TYPENAME:
      type: string
      x-go-type: githubrepo/lib/pkg.{TYPENAME}
      x-go-type-string-parse: githubrepo/lib/pkg.Parse{TYPENAME}
```

If you want to have a specific Go map, you can also use `x-go-type` to specify a key. It works only if `additionalProperties` is specified:
```yaml
    ResponseBody:
      type: object
      additionalProperties:
        items:
          $ref: '#/components/schemas/objects.Type'
        type: array
      x-go-type: githubrepo/objects.SomeType
```

Use `x-go-string-trimmable` key if you would like to trim spaces before validation. It works only for string type:
```yaml
    ResponseBody:
      properties:
        title:
          type: string
          x-go-string-trimmable: true
```

If you want to add `omitempty` tag, you can also use `x-go-omitempty`:
```yaml
    ResponseBody:
      properties:
        title:
          type: string
          x-go-omitempty: true
```

By default, validation is added to request body objects. If you want to ignore validation, use the `x-go-skip-validation` flag:
```yaml
    ResponseBody:
      properties:
        title:
          type: string
      x-go-skip-validation: true
```
## Questions or Feature Requests

Have a question or need some functionality? Feel free to open an issue or submit a pull request.

## Contributing

Go OpenAPI v3 server code generator uses [github.com/dave/jennifer](https://github.com/dave/jennifer) for code generation. 
Using [github.com/aloder/tojen](https://github.com/aloder/tojen) is the suggested way to generate Jennifer code.
