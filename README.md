Go OpenAPI v3 server codegenerator
----------------------------------------
The purpose of this project is to generate a clean server boilerplate code from openapi v3 specification. The generated code based on github.com/go-chi/chi/v5 router. Generator goes over all paths and components and generates Go structs and stubs. 

#### Key ideas:
- Stubs take over the logic of parsing the request.
- Response builders encapsulated logic that doesn't allow you to respond differently from your specification.

Take a note that path stubs generation relies on the **first tag** from your paths.
## Installation
```
GO111MODULE=on go get github.com/mikekonan/go-oas3@v1.0.53
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
  -prioritize-x-go-type
    by default, if both properties and x-go-type is provided, the generator will use properties.
    this flag will make generator prioritize x-go-type over properties.

```
## Example
Run with: ```go-oas3 -swagger-addr https://raw.githubusercontent.com/mikekonan/go-oas3/v1.0.53/example/swagger.yaml -package example -path ./example```
The result generated boilerplate and its client you can see at ./example.

# OpenAPI features
### Required fields
Path, query, component, header required fields are supported.

### Security
Security schemas for http, apikey (header/cookie).

### Cookie
Response header `Set-Cookie` supported. *Cookie in request supported via security schema only.*  

### Validation
Types validation supports following data types:
- **string**: minLength, maxLength
- **number**, **integer**: minimum, maximum, exclusiveMinimum, exclusiveMaximum

### Custom types
Generator supports few swagger types for components. 
|openapi type|go type|
|---|---|
|uuid|github.com/google/uuid.UUID|
|iso4217-currency-code|github.com/mikekonan/go-types/v2/currency.Code|
|iso3166-alpha-2|github.com/mikekonan/go-types/v2/country.Alpha2Code|
|iso3166-alpha-3|github.com/mikekonan/go-types/v2/country.Alpha3Code|

#### Extentions:
Specify a go type with:
```
    Component:
      properties:
        metadata:
          type: object
          x-go-type: encoding/json.RawMessage
```

Forcing pointer usage for the field:
```
    Component:
      properties:
        amount:
          type: int
          x-go-pointer: true
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

If you want to have a specific go map you can also use `x-go-type` to specify a key. It works only if additionalProperties specified.
```
    ResponseBody:
      type: object
      additionalProperties:
        items:
          $ref: '#/components/schemas/objects.Type'
        type: array
      x-go-type: githubrepo/objects.SomeType
```

Use `x-go-string-timmable` key If you would like to trim spaces before validation. It works only for string type
```
    ResponseBody:
      properties:
        title:
          type: string
          x-go-string-trimmable: true
```

If you want to add omitempty tag you can also use `x-go-omitempty`
```
    ResponseBody:
      properties:
        title:
          type: string
          x-go-omitempty: true
```

By default, validation is added to request body objects. If you want to ignore validation, use flag `x-go-skip-validation`
```
    ResponseBody:
      properties:
        title:
          type: string
      x-go-skip-validation: true
```
## Have a question or need some functionality?
Feel free to discuss it or do a PR.

## Contribution
Go OpenAPI v3 server codegenerator uses https://github.com/dave/jennifer. 
Using https://github.com/aloder/tojen is suggested way to generate jennifer code.
