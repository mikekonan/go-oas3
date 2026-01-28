# Fix: x-go-omitempty ignored when field described with $ref

**Issue:** [#106](https://github.com/mikekonan/go-oas3/issues/106)

## Problem

The `x-go-omitempty` extension was being ignored when a field was defined using `$ref` to reference another schema:

```yaml
MyCoolStructure:
  properties:
    myFieldName:
      $ref: '#/components/schemas/MyType'
      x-go-omitempty: true
```

**Expected output:**
```go
type MyCoolStructure struct {
    MyFieldName MyType `json:"myFieldName,omitempty"`
}
```

**Actual output (before fix):**
```go
type MyCoolStructure struct {
    MyFieldName MyType `json:"myFieldName"`
}
```

## Root Cause

The `kin-openapi` library (v0.112.0) did not support storing extensions placed alongside `$ref` in OpenAPI schemas. The `SchemaRef` struct only had `Ref` and `Value` fields, with no `Extensions` field.

Starting from **v0.126.0**, `kin-openapi` added support for `SchemaRef.Extensions` which stores extensions placed next to `$ref`.

## Solution

### 1. Updated kin-openapi dependency

Upgraded from `v0.112.0` to `v0.133.0` to get `SchemaRef.Extensions` support.

### 2. Adapted code to breaking changes in kin-openapi v0.133.0

| Change | Before | After |
|--------|--------|-------|
| `Paths` type | `map[string]*PathItem` | `*Paths` struct with `.Map()` method |
| `Responses` type | `map[string]*ResponseRef` | `*Responses` struct with `.Map()` and `.Len()` methods |
| `Schema.Type` | `string` | `*Types` (use `.Is(typ)` method) |
| `AdditionalProperties` | `*SchemaRef` | struct with `Has *bool` and `Schema *SchemaRef` |
| Extensions values | `json.RawMessage` | native Go types (`string`, `bool`, etc.) |

### 3. Added support for x-go-omitempty on $ref fields

New function `getXGoOmitemptyFromSchemaRef` checks for `x-go-omitempty` in both:
- `SchemaRef.Extensions` - for extensions placed alongside `$ref`
- `Schema.Extensions` - for inline schemas (existing behavior)

## Files Changed

### Core changes
- `go.mod`, `go.sum` - dependency update
- `generator/type.go` - helper functions and omitempty fix
- `generator/generator.go` - adaptation to new kin-openapi API

### Regenerated examples
- `example/*_gen.go`
- `example/arraytest/*_gen.go`
- `example/minimal/*_gen.go`
- `example/simplearray/*_gen.go`

## Testing

### Verified fix works

Test schema:
```yaml
MyCoolStructure:
  properties:
    myFieldWithRef:
      $ref: '#/components/schemas/MyType'
      x-go-omitempty: true
```

Generated output:
```go
type MyCoolStructure struct {
    MyFieldWithRef MyType `json:"myFieldWithRef,omitempty"`
}
```

### Regression testing

- All example specs regenerate successfully
- Generated code compiles
- Note: Some arraytest validation tests fail, but this is a pre-existing issue unrelated to this fix (the generator doesn't support `minItems`/`maxItems` array validation)

## Breaking Changes

None for users of the generator. The generated code API remains the same.

## Migration Notes

If you have custom code that depends on `kin-openapi` types directly, you may need to update for the new API:
- Use `swagger.Paths.Map()` instead of iterating over `swagger.Paths` directly
- Use `schema.Type.Is("string")` instead of `schema.Type == "string"`
