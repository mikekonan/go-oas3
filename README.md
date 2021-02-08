Go OpenAPI v3 server codegen
----------------------------------------
The purpose of this project is to generate a clean server boilerplate code from openapi v3 specification. The generated code based on github.com/go-chi/chi router. Generator goes over all paths and components and generates Go structs and stubs. 

#### Key ideas:
- Stubs take over the logic of parsing the request.
- Response builders encapsulated logic that doesn't allow you to respond differently from your specification.

Take a note that path stubs generation relies on the **first tag** from your paths.
## Installation
```
GO111MODULE=on go get github.com/mikekonan/go-oas3@v1.0.12
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
```
## Example
Run with: ```go-oas3 -swagger-addr https://raw.githubusercontent.com/OAI/OpenAPI-Specification/master/examples/v3.0/petstore.yaml -package example -path ./example```
The result generated boilerplate and its client you can see at ./example.

# OpenAPI features
### Required fields
Path, query, component, header required fields are supported. Security schemas for http and apikey(header).

### Custom types
Generator supports few swagger types for components. 
|openapi type|go type|
|---|---|
|uuid|github.com/google/uuid.UUID|
|iso4217-currency-code|github.com/mikekonan/go-currencies.Code|
|iso3166-alpha-2|github.com/mikekonan/go-countries.Alpha2Code|
|iso3166-alpha-3|github.com/mikekonan/go-countries.Alpha3Code|

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

## Plans
- [ ] Support more types.

- [x] Support security schemas. Cookies - left.

- [ ] Remove cast dependency from generated code.

- [ ] Create an example that covers all use cases.

- [x] Added handler that provides spec in json format.

## Have a question or need some functionality?
Feel free to discuss it or do a PR.
