Go OpenAPI v3 server codegenerator
----------------------------------------
The purpose of this project is to generate a clean server boilerplate code from openapi v3 specification. The generated code based on github.com/go-chi/chi router. Generator goes over all paths and components and generates Go structs and stubs. 

#### Key ideas:
- Stubs take over the logic of parsing the request.
- Response builders encapsulated logic that doesn't allow you to respond differently from your specification.

Take a note that path stubs generation relies on the **first tag** from your paths.
## Installation
```
GO111MODULE=on go get github.com/mikekonan/go-oas3@v1.0.32
```
## Program arguments
```
Usage of go-oas3:
  -componentsPackage string
  -componentsPath string
  -package string
  -path string
  -swagger-addr string
    	 (default "swagger.yaml")
  -authorization string 
    a list of comma-separated key:value pairs to be sent as headers alongside each http request

```
## Example
Run with: ```go-oas3 -swagger-addr https://raw.githubusercontent.com/OAI/OpenAPI-Specification/master/examples/v3.0/petstore.yaml -package example -path ./example```
The result generated boilerplate and its client you can see at ./example.

# OpenAPI features
### Required fields
Path, query, component, header required fields are supported. Security schemas for http and apikey(header).

### Validation
Types validation supports following data types:
- **string**: minLength, maxLength
- **number**, **integer**: minimum, maximum, exclusiveMinimum, exclusiveMaximum

### Custom types
Generator supports few swagger types for components. 
|openapi type|go type|
|---|---|
|uuid|github.com/google/uuid.UUID|
|iso4217-currency-code|github.com/mikekonan/go-types/currency.Code|
|iso3166-alpha-2|github.com/mikekonan/go-types/country.Alpha2Code|
|iso3166-alpha-3|github.com/mikekonan/go-types/country.Alpha3Code|

#### Extentions:
Specify a go type with:
```
    Component:
      properties:
        metadata:
          type: object
          x-go-type: encoding/json.RawMessage
```

Specify a regex to match a string:
```
    Parameter:
      description: Parameter
      in: header
      name: parameter name
      required: true
      schema:
        type: string
        x-go-regex: ^[.?\d]+$
```

If you want to use your specific type(it has to declare function ```Parse{TYPENAME} ({TYPENAME}, error)```) in query/path/header params:
```
    TYPENAME:
      type: string
      x-go-type: githubrepo/lib/pkg.{TYPENAME}
      x-go-type-string-parse: githubrepo/lib/pkg.Parse{TYPENAME}
```

## Plans

- [ ] Support cookies in security schemas.

## Have a question or need some functionality?
Feel free to discuss it or do a PR.

## Contribution
Go OpenAPI v3 server codegenerator uses https://github.com/dave/jennifer. 
Using https://github.com/aloder/tojen is suggested way to generate jennifer code.
