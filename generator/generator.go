package generator

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/tdewolff/minify/v2/minify"

	"github.com/mikekonan/go-oas3/configurator"
)

type Generator struct {
	normalizer *Normalizer
	typee      *Type
	config     *configurator.Config

	// optimize code generator for regexp
	useRegex map[string]string

	// errVars dedupes runtime error messages into package-level vars so each
	// unique message allocates once, not per call.
	componentErrVars map[string]string
	routesErrVars    map[string]string
}

type errFile int

const (
	errFileComponents errFile = iota
	errFileRoutes
)

// errVar returns the identifier for the package-level error variable
// matching msg in the target file, registering a new one on first sight.
// When components and routes share the same Go package the maps are
// coalesced so the var block isn't declared twice.
func (generator *Generator) errVar(msg string, file errFile) string {
	if generator.errVarsPackagesShared() {
		file = errFileComponents
	}
	m := generator.errVarsMap(file)
	if name, ok := (*m)[msg]; ok {
		return name
	}
	base := buildErrVarName(msg, len(*m))
	// Different messages can normalize to the same identifier ("x is required"
	// vs "x_is_required"). Walk an incrementing suffix until we find one no
	// other entry claims — a single check isn't enough because the chosen
	// suffix itself can collide with an earlier <name><digits> entry.
	used := make(map[string]struct{}, len(*m))
	for _, existing := range *m {
		used[existing] = struct{}{}
	}
	name := base
	for i := 1; ; i++ {
		if _, taken := used[name]; !taken {
			break
		}
		name = fmt.Sprintf("%s%d", base, i)
	}
	(*m)[msg] = name
	return name
}

// errVarsPackagesShared reports whether components and routes end up in the
// same Go package — i.e. the same on-disk directory AND the same package
// identifier. Matching just the package identifier is not enough: two
// directories that both declare `package gen` are still two distinct Go
// packages, so coalescing error vars into a single declaration would leave
// the other file with unresolved references.
func (generator *Generator) errVarsPackagesShared() bool {
	return generator.config.ComponentsPackage == generator.config.Package &&
		generator.config.ComponentsPath == generator.config.Path
}

func (generator *Generator) errVarsMap(file errFile) *map[string]string {
	if file == errFileComponents {
		if generator.componentErrVars == nil {
			generator.componentErrVars = map[string]string{}
		}
		return &generator.componentErrVars
	}
	if generator.routesErrVars == nil {
		generator.routesErrVars = map[string]string{}
	}
	return &generator.routesErrVars
}

// errVarsBlock emits the `var (...)` block of registered errors for file.
// When components and routes share a package the routes-side block is
// empty — the declarations live in components_gen.go.
func (generator *Generator) errVarsBlock(file errFile) jen.Code {
	if generator.errVarsPackagesShared() && file == errFileRoutes {
		return jen.Null()
	}
	m := *generator.errVarsMap(file)
	if len(m) == 0 {
		return jen.Null()
	}
	type entry struct{ name, msg string }
	entries := make([]entry, 0, len(m))
	for msg, name := range m {
		entries = append(entries, entry{name, msg})
	}
	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.name, b.name) })
	defs := make([]jen.Code, 0, len(entries))
	for _, e := range entries {
		defs = append(defs, jen.Id(e.name).Op("=").Qual("errors", "New").Call(jen.Lit(e.msg)))
	}
	return jen.Var().Defs(defs...).Line()
}

// commonInitialisms — words emitted all-caps when standalone in an identifier.
var commonInitialisms = map[string]string{
	"id":    "ID",
	"uuid":  "UUID",
	"http":  "HTTP",
	"https": "HTTPS",
	"url":   "URL",
	"uri":   "URI",
	"api":   "API",
	"json":  "JSON",
	"xml":   "XML",
	"html":  "HTML",
	"sql":   "SQL",
	"ip":    "IP",
}

// buildErrVarName turns a message into an err-prefixed Go identifier.
// Non-alphanumeric runes split words; initialisms render all-caps.
// Falls back to errN when normalization yields nothing.
func buildErrVarName(msg string, idx int) string {
	var sb strings.Builder
	sb.WriteString("err")
	var word strings.Builder
	flush := func() {
		if word.Len() == 0 {
			return
		}
		w := word.String()
		if up, ok := commonInitialisms[strings.ToLower(w)]; ok {
			sb.WriteString(up)
		} else {
			sb.WriteString(strings.ToUpper(w[:1]))
			sb.WriteString(w[1:])
		}
		word.Reset()
	}
	for _, r := range msg {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			word.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	if sb.Len() == 3 {
		return fmt.Sprintf("err%d", idx)
	}
	return sb.String()
}

type Result struct {
	ComponentsCode *jen.File
	RouterCode     *jen.File
	SpecCode       *jen.File
}

func New(normalizer *Normalizer, typee *Type, config *configurator.Config) *Generator {
	return &Generator{
		normalizer: normalizer,
		typee:      typee,
		config:     config,
	}
}

func NewNormalizer() *Normalizer { return &Normalizer{} }

func NewType(normalizer *Normalizer, config *configurator.Config) *Type {
	return &Type{normalizer: normalizer, config: config}
}

// inlineResponseBodyTypeName builds the Go identifier for an inline response
// body schema. The status code MUST be part of the name: an operation can
// legally declare different inline schemas under the same content-type for
// different status codes (e.g. 200/application/json and 400/application/json),
// and dropping the status would silently coalesce them in components_gen.go.
// All three callers — components(), builders(), genericResponseBodyTypeName()
// — share this helper so the emitted struct and the type referenced from the
// service signature always agree.
func (generator *Generator) inlineResponseBodyTypeName(operationName, statusCode, contentType string) string {
	return operationName + statusCode + strings.Title(generator.normalizer.normalize(contentType))
}

// sortedMapKeys returns sorted keys from any map to ensure deterministic iteration
func sortedMapKeys[K ~string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// sortedKeyValue represents a key-value pair for sorted map iteration
type sortedKeyValue[K comparable, V any] struct {
	Key   K
	Value V
}

// mustParseStatusCode panics on a non-integer status; the spec guarantees
// "100".."599" here and we'd rather fail loudly than emit 0.
func mustParseStatusCode(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(fmt.Sprintf("status code %q is not an integer: %v", s, err))
	}
	return n
}

// sortedMapEntries returns sorted key-value pairs from any map to ensure deterministic iteration
func sortedMapEntries[K ~string, V any](m map[K]V) []sortedKeyValue[K, V] {
	keys := sortedMapKeys(m)
	entries := make([]sortedKeyValue[K, V], len(keys))
	for i, k := range keys {
		entries[i] = sortedKeyValue[K, V]{Key: k, Value: m[k]}
	}
	return entries
}

func (generator *Generator) file(from jen.Code, importPath, packageName string) *jen.File {
	file := jen.NewFilePathName(importPath, packageName)
	file.HeaderComment("This file is generated by github.com/mikekonan/go-oas3. DO NOT EDIT.")
	file.ImportAlias("github.com/mikekonan/go-types/v2/country", "countries")
	file.ImportAlias("github.com/mikekonan/go-types/v2/currency", "currency")
	file.ImportAlias("github.com/go-ozzo/ozzo-validation/v4", "validation")
	file.ImportAlias("github.com/go-chi/chi/v5", "chi")

	file.Add(from)

	return file
}

func (generator *Generator) Generate(swagger *openapi3.T) *Result {
	componentsAdditionalVars, parametersAdditionalVars := generator.additionalConstants(swagger)

	// Generate bodies first so errVar registrations are complete before
	// errVarsBlock emits the var declarations.
	componentsBody := generator.components(swagger)
	wrappersBody := generator.wrappers(swagger)
	buildersBody := generator.requestResponseBuilders(swagger)
	securityBody := generator.securitySchemas(swagger)

	componentsCode := jen.Null().
		Add(componentsAdditionalVars).
		Add(generator.errVarsBlock(errFileComponents)).
		Add(componentsBody)
	routerCode := jen.Null().
		Add(parametersAdditionalVars...).Line().
		Add(generator.errVarsBlock(errFileRoutes)).
		Add(wrappersBody).Line().
		Add(buildersBody).Line().
		Add(securityBody)

	// `package X` follows the directory basename, not the import-path basename
	// — `api/v1/gen` directory is `package gen`, even though Qual imports it
	// as `api/v1/gen`.
	routerPkgName := generator.trimPackagePath(generator.config.Path)
	componentsPkgName := generator.trimPackagePath(generator.config.ComponentsPath)

	return &Result{
		ComponentsCode: generator.file(componentsCode, generator.config.ComponentsPackage, componentsPkgName),
		RouterCode:     generator.file(routerCode, generator.config.Package, routerPkgName),
		SpecCode:       generator.file(generator.specCode(swagger), generator.config.Package, routerPkgName),
	}
}

func (generator *Generator) requestParameters(paths map[string]*openapi3.PathItem) jen.Code {
	codeByTag := map[string][]jen.Code{}
	for _, pathEntry := range sortedMapEntries(paths) {
		path := pathEntry.Key
		for _, opEntry := range sortedMapEntries(pathEntry.Value.Operations()) {
			operation := opEntry.Value
			tag := generator.normalizer.normalize("default")
			if len(operation.Tags) > 0 {
				tag = generator.normalizer.normalize(operation.Tags[0])
			}

			name := generator.normalizer.normalizeOperationName(path, opEntry.Key)

			var opCode []jen.Code
			switch {
			case operation.RequestBody == nil:
				opCode = append(opCode, generator.requestParameterStruct(name, "", false, operation))
			case len(operation.RequestBody.Value.Content) == 1:
				contentType := sortedMapKeys(operation.RequestBody.Value.Content)[0]
				opCode = append(opCode, generator.requestParameterStruct(name, contentType, false, operation))
			default:
				for _, ctEntry := range sortedMapEntries(operation.RequestBody.Value.Content) {
					opCode = append(opCode, generator.requestParameterStruct(name, ctEntry.Key, true, operation))
				}
				opCode = generator.normalizer.doubleLineAfterEachElement(opCode...)
			}
			codeByTag[tag] = append(codeByTag[tag], opCode...)
		}
	}

	tags := sortedMapKeys(codeByTag)

	out := jen.Null()
	for _, tag := range tags {
		out = out.Add(generator.normalizer.lineAfterEachElement(codeByTag[tag]...)...)
	}
	return out.Add(jen.Line())
}

func (generator *Generator) components(swagger *openapi3.T) jen.Code {
	var componentsResult []jen.Code

	// Sort schema names to ensure deterministic component generation order
	var schemaNames []string
	for schemaName, schemaRef := range swagger.Components.Schemas {
		if len(schemaRef.Value.Enum) == 0 { // filter enums
			schemaNames = append(schemaNames, schemaName)
		}
	}
	slices.Sort(schemaNames)

	for _, schemaName := range schemaNames {
		schemaRef := swagger.Components.Schemas[schemaName]
		componentsResult = append(componentsResult, generator.componentFromSchema(schemaName, schemaRef))
	}

	// Inline request-body components: one struct per operation that has an
	// inline (non-$ref) content entry. Single-content-type → <Op>RequestBody;
	// multi-content-type → <Op><ContentType>RequestBody per entry. Dedupe by
	// name before emitting so duplicates produce one declaration.
	componentsByName := map[string]jen.Code{}
	for _, pathEntry := range sortedMapEntries(swagger.Paths.Map()) {
		path := pathEntry.Key
		for _, opEntry := range sortedMapEntries(pathEntry.Value.Operations()) {
			operation := opEntry.Value
			if operation.RequestBody == nil || len(operation.RequestBody.Value.Content) == 0 {
				continue
			}
			hasInline := false
			for _, mt := range operation.RequestBody.Value.Content {
				if mt.Schema.Ref == "" {
					hasInline = true
					break
				}
			}
			if !hasInline {
				continue
			}

			baseName := generator.normalizer.normalizeOperationName(path, opEntry.Key)
			if len(operation.RequestBody.Value.Content) == 1 {
				name := baseName + "RequestBody"
				schema := sortedMapEntries(operation.RequestBody.Value.Content)[0].Value.Schema
				componentsByName[name] = generator.componentFromSchema(name, schema)
				continue
			}
			for _, ctEntry := range sortedMapEntries(operation.RequestBody.Value.Content) {
				objName := baseName + generator.normalizer.contentType(ctEntry.Key+"RequestBody")
				componentsByName[objName] = generator.componentFromSchema(objName, ctEntry.Value.Schema)
			}
		}
	}

	componentsFromPathsResult := make([]jen.Code, 0, len(componentsByName))
	for _, entry := range sortedMapEntries(componentsByName) {
		componentsFromPathsResult = append(componentsFromPathsResult, entry.Value)
	}

	// Inline response-body components: dedupe by name and emit in sorted order.
	inlineByName := map[string]jen.Code{}
	for _, pathEntry := range sortedMapEntries(swagger.Paths.Map()) {
		for _, opEntry := range sortedMapEntries(pathEntry.Value.Operations()) {
			operation := opEntry.Value
			if operation.Responses == nil || operation.Responses.Len() == 0 {
				continue
			}
			operationName := generator.normalizer.normalizeOperationName(pathEntry.Key, opEntry.Key)
			for _, respEntry := range sortedMapEntries(operation.Responses.Map()) {
				if len(respEntry.Value.Value.Content) == 0 {
					continue
				}
				for _, ctEntry := range sortedMapEntries(respEntry.Value.Value.Content) {
					if ctEntry.Value.Schema.Ref != "" {
						continue
					}
					objName := generator.inlineResponseBodyTypeName(operationName, respEntry.Key, ctEntry.Key)
					inlineByName[objName] = generator.componentFromSchema(objName, ctEntry.Value.Schema)
				}
			}
		}
	}
	for _, entry := range sortedMapEntries(inlineByName) {
		componentsFromPathsResult = append(componentsFromPathsResult, entry.Value)
	}

	componentsResult = generator.normalizer.doubleLineAfterEachElement(componentsResult...)

	componentsFromPathsResult = generator.normalizer.doubleLineAfterEachElement(componentsFromPathsResult...)

	return jen.Null().
		Add(componentsResult...).
		Add(jen.Line()).
		Add(componentsFromPathsResult...).
		Add(jen.Line()).
		Add(generator.enums(swagger)).
		Add(jen.Line())
}

func (generator *Generator) variableForRegex(name string, schema *openapi3.SchemaRef) jen.Code {
	hasGoRegexExtension := len(schema.Value.Extensions) > 0 && schema.Value.Extensions[goRegex] != nil

	if !hasGoRegexExtension || !isSchemaType(schema.Value.Type, "string") {
		return jen.Null()
	}

	regex := parseExtensionString(schema.Value.Extensions[goRegex])

	if generator.useRegex == nil {
		generator.useRegex = map[string]string{}
	}

	if _, ok := generator.useRegex[regex]; ok {
		return jen.Empty()
	}

	name = generator.normalizer.decapitalize(name) + "Regex"
	generator.useRegex[regex] = name
	return jen.Var().
		Id(name).
		Op("=").
		Qual("regexp", "MustCompile").
		Call(jen.Lit(regex)).
		Line()
}

func (generator *Generator) additionalConstants(swagger *openapi3.T) (jen.Code, []jen.Code) {
	var constantsComponentsCode []jen.Code

	// Sort schema names to ensure deterministic constants generation order
	var schemaNames []string
	for schemaName := range swagger.Components.Schemas {
		schemaNames = append(schemaNames, schemaName)
	}
	slices.Sort(schemaNames)

	for _, schemaName := range schemaNames {
		schema := swagger.Components.Schemas[schemaName]

		// Sort property names to ensure deterministic property constants generation order
		var propNames []string
		for propName := range schema.Value.Properties {
			propNames = append(propNames, propName)
		}
		slices.Sort(propNames)

		for _, propName := range propNames {
			propSchema := schema.Value.Properties[propName]
			name := generator.normalizer.normalize(strings.Title(propName))
			constantsComponentsCode = append(constantsComponentsCode, generator.variableForRegex(name, propSchema))
		}
	}

	var componentsPathsCode []jen.Code

	// Sort paths to ensure deterministic path constants generation order
	var pathNames []string
	for pathName := range swagger.Paths.Map() {
		pathNames = append(pathNames, pathName)
	}
	slices.Sort(pathNames)

	for _, pathName := range pathNames {
		pathItem := swagger.Paths.Value(pathName)

		// Sort operations to ensure deterministic operation constants generation order
		var operationMethods []string
		for method := range pathItem.Operations() {
			operationMethods = append(operationMethods, method)
		}
		slices.Sort(operationMethods)

		for _, method := range operationMethods {
			operation := pathItem.Operations()[method]
			if operation.RequestBody == nil || len(operation.RequestBody.Value.Content) == 0 {
				continue
			}

			// Check if any content has inline schema (no ref)
			hasInlineSchema := false
			for _, mediaType := range operation.RequestBody.Value.Content {
				if mediaType.Schema.Ref == "" {
					hasInlineSchema = true
					break
				}
			}

			if !hasInlineSchema {
				continue
			}

			name := generator.normalizer.normalizeOperationName(pathName, method)
			name = generator.normalizer.decapitalize(name)

			// Sort content types to ensure deterministic content constants generation order
			var contentTypes []string
			for contentType := range operation.RequestBody.Value.Content {
				contentTypes = append(contentTypes, contentType)
			}
			slices.Sort(contentTypes)

			for _, contentType := range contentTypes {
				meType := operation.RequestBody.Value.Content[contentType]

				// Sort property names to ensure deterministic property constants generation order
				var propNames []string
				for propName := range meType.Schema.Value.Properties {
					propNames = append(propNames, propName)
				}
				slices.Sort(propNames)

				for _, propName := range propNames {
					propSchema := meType.Schema.Value.Properties[propName]
					name := generator.normalizer.normalize(strings.Title(propName))
					componentsPathsCode = append(componentsPathsCode, generator.variableForRegex(name, propSchema))
				}
			}
		}
	}

	componentsCode := jen.Null().
		Add(constantsComponentsCode...).
		Line().
		Add(componentsPathsCode...).
		Line()

	var parametersCode []jen.Code

	// Sort paths to ensure deterministic parameter constants generation order
	var pathNames2 []string
	for pathName := range swagger.Paths.Map() {
		pathNames2 = append(pathNames2, pathName)
	}
	slices.Sort(pathNames2)

	for _, pathName := range pathNames2 {
		pathItem := swagger.Paths.Value(pathName)

		// Sort operations to ensure deterministic operation parameter constants generation order
		var operationMethods2 []string
		for method := range pathItem.Operations() {
			operationMethods2 = append(operationMethods2, method)
		}
		slices.Sort(operationMethods2)

		for _, method := range operationMethods2 {
			operation := pathItem.Operations()[method]
			name := generator.normalizer.normalizeOperationName(pathName, method)
			name = generator.normalizer.decapitalize(name)

			for _, parameter := range operation.Parameters {
				parametersCode = append(parametersCode,
					generator.variableForRegex(generator.normalizer.normalize(strings.Title(parameter.Value.Name)), parameter.Value.Schema))
			}
		}
	}

	return componentsCode, parametersCode
}

func (generator *Generator) requestParameterStruct(name string, contentType string, appendContentTypeToName bool, operation *openapi3.Operation) jen.Code {
	type parameter struct {
		In   string
		Code jen.Code
	}

	var additionalParameters []parameter

	if contentType != "" {
		if appendContentTypeToName {
			name += generator.normalizer.contentType(contentType)
		}

		bodyTypeName := generator.normalizer.extractNameFromRef(operation.RequestBody.Value.Content[contentType].Schema.Ref)
		if bodyTypeName == "" {
			bodyTypeName = name + "RequestBody"
		}

		additionalParameters = append(additionalParameters,
			parameter{In: "Body", Code: jen.Id("Body").Qual(generator.config.ComponentsPackage, bodyTypeName)})
	}

	paramsByIn := map[string][]*openapi3.ParameterRef{}
	for _, p := range operation.Parameters {
		paramsByIn[p.Value.In] = append(paramsByIn[p.Value.In], p)
	}
	ins := sortedMapKeys(paramsByIn)
	for _, group := range paramsByIn {
		slices.SortFunc(group, func(a, b *openapi3.ParameterRef) int {
			return strings.Compare(a.Value.Name, b.Value.Name)
		})
	}

	parameterStructs := make([]jen.Code, 0, len(ins))
	for _, in := range ins {
		group := paramsByIn[in]
		typeName := name + "Request" + strings.Title(in)

		structFields := make([]jen.Code, 0, len(group))
		getters := make([]jen.Code, 0, len(group))
		var fieldValidationRules []jen.Code

		for _, parameter := range group {
			pName := generator.normalizer.normalize(parameter.Value.Name)

			field := jen.Id(pName)
			if len(parameter.Value.Schema.Value.Enum) > 0 && len(parameter.Value.Schema.Ref) > 0 {
				generator.typee.fillGoType(field, "", generator.normalizer.extractNameFromRef(parameter.Value.Schema.Ref), parameter.Value.Schema, false, false)
			} else {
				generator.typee.fillGoType(field, "", pName, parameter.Value.Schema, false, false)
				// Header gets a json tag so ozzo-validation reports the on-wire
				// header name in error messages.
				if parameter.Value.In == "header" {
					generator.typee.fillJsonTag(field, parameter.Value.Schema, parameter.Value.Name)
				}
			}
			structFields = append(structFields, field)

			fvRule := generator.fieldValidationRuleFromSchema(parameter.Value.In, pName, parameter.Value.Schema, parameter.Value.Required)
			if fvRule != nil {
				fieldValidationRules = append(fieldValidationRules, jen.Line().Add(fvRule))
			}

			getter := jen.Func().Params(jen.Id(parameter.Value.In).Id(typeName)).Id("Get" + pName).Params()
			returnType := jen.Null()
			if len(parameter.Value.Schema.Value.Enum) > 0 && len(parameter.Value.Schema.Ref) > 0 {
				generator.typee.fillGoType(returnType, "", generator.normalizer.extractNameFromRef(parameter.Value.Schema.Ref), parameter.Value.Schema, false, false)
			} else {
				generator.typee.fillGoType(returnType, "", pName, parameter.Value.Schema, false, false)
			}
			getter = getter.Params(returnType).Block(jen.Return().Id(parameter.Value.In).Dot(pName))
			getters = append(getters, getter)
		}

		validateFunc := generator.validationFuncFromRules(in, typeName, fieldValidationRules, nil)
		parameterStructs = append(parameterStructs,
			jen.Type().Id(typeName).Struct(structFields...).
				Line().Line().
				Add(generator.normalizer.doubleLineAfterEachElement(getters...)...).
				Add(validateFunc))
	}

	type inField struct {
		in   string
		code jen.Code
	}
	allFields := make([]inField, 0, len(ins)+len(additionalParameters))
	for _, in := range ins {
		title := strings.Title(in)
		allFields = append(allFields, inField{
			in:   in,
			code: jen.Id(title).Id(name + "Request" + title),
		})
	}
	for _, ap := range additionalParameters {
		allFields = append(allFields, inField{in: ap.In, code: ap.Code})
	}
	slices.SortFunc(allFields, func(a, b inField) int { return strings.Compare(a.in, b.in) })

	parameters := make([]jen.Code, 0, len(allFields))
	for _, f := range allFields {
		parameters = append(parameters, f.code)
	}

	useGen := generator.useGenerics(operation)
	hasSecuritySchemas := operation.Security != nil && len(*operation.Security) > 0

	if useGen {
		// Generic operations embed RequestMeta so ProcessingResult and
		// SecurityCheckResults plus their accessors come from one shared type.
		parameters = append(parameters, jen.Id("RequestMeta"))
	} else {
		parameters = append(parameters, jen.Id("ProcessingResult").Id("RequestProcessingResult"))
		if hasSecuritySchemas {
			parameters = append(parameters, jen.Id("SecurityCheckResults").Map(jen.Id("SecurityScheme")).Id("string"))
		}
	}

	hasHeader := false
	for _, p := range operation.Parameters {
		if p.Value.In == "header" {
			hasHeader = true
			break
		}
	}

	result := jen.Null().
		Add(generator.normalizer.doubleLineAfterEachElement(parameterStructs...)...).
		Line().Line().
		Add(jen.Type().Id(name + "Request").Struct(parameters...)).
		Line().Line()

	if useGen {
		// GetHeader is per-operation because Header is uniquely typed per
		// endpoint; the other Request-interface methods come from RequestMeta.
		result = result.Add(generator.requestHeaderAccessor(name+"Request", hasHeader)).Line().Line()
	}

	return result
}

func (generator *Generator) requestHeaderAccessor(typeName string, hasHeader bool) jen.Code {
	receiver := jen.Id("r").Id(typeName)
	if hasHeader {
		return jen.Func().Params(receiver).
			Id("GetHeader").Params().Params(jen.Any()).
			Block(jen.Return().Id("r").Dot("Header"))
	}
	return jen.Func().Params(receiver).
		Id("GetHeader").Params().Params(jen.Any()).
		Block(jen.Return().Nil())
}

func (generator *Generator) enumFromSchema(name string, schema *openapi3.SchemaRef) jen.Code {
	if len(schema.Ref) > 0 {
		return jen.Null()
	}

	name = generator.normalizer.normalize(name)
	v := schema.Value

	// Check if this enum should be aliased to an external type instead of generating constants
	if generator.shouldUseExternalTypeAlias(v) {
		return generator.generateExternalTypeAlias(name, schema)
	}

	var result []jen.Code
	result = append(result, jen.Type().Id(generator.normalizer.normalize(name)).String())

	enumValues := make([]jen.Code, 0, len(schema.Value.Enum))
	enumSwitchCases := make([]jen.Code, 0, len(schema.Value.Enum))
	for _, raw := range schema.Value.Enum {
		value := raw.(string)
		valName := name + generator.normalizer.normalize(strings.Title(value))
		enumValues = append(enumValues, jen.Var().Id(valName).Id(name).Op("=").Lit(value))
		enumSwitchCases = append(enumSwitchCases, jen.Id(valName))
	}

	result = append(result, enumValues...)

	result = append(result, jen.Func().Params(
		jen.Id("enum").Id(name)).Id("Check").Params().Params(
		jen.Id("error")).Block(
		jen.Switch(jen.Id("enum")).Block(
			jen.Case(enumSwitchCases...).Block(
				jen.Line().Return().Id("nil"))),
		jen.Line().Return().Id(generator.errVar(fmt.Sprintf("invalid %s enum value", name), errFileComponents)),
	).Add(jen.Line()))

	result = append(result, jen.Func().Params(
		jen.Id("enum").Op("*").Id(name)).Id("UnmarshalJSON").Params(
		jen.Id("data").Index().Id("byte")).Params(
		jen.Id("error")).Block(
		jen.Var().Id("strValue").Id("string"),
		jen.If(jen.Id("err").Op(":=").Qual("encoding/json",
			"Unmarshal").Call(jen.Id("data"),
			jen.Op("&").Id("strValue")),
			jen.Id("err").Op("!=").Id("nil")).Block(
			jen.Line().Return().Id("err")),
		jen.Id("enumValue").Op(":=").Id(name).Call(jen.Id("strValue")),
		jen.If(jen.Id("err").Op(":=").Id("enumValue").Dot("Check").Call(),
			jen.Id("err").Op("!=").Id("nil")).Block(
			jen.Line().Return().Id("err")),
		jen.Op("*").Id("enum").Op("=").Id("enumValue"),
		jen.Line().Return().Id("nil"),
	))

	result = generator.normalizer.lineAfterEachElement(result...)

	return jen.Null().Add(result...).Add(jen.Line())
}

func (generator *Generator) getXGoRegex(schema *openapi3.SchemaRef) string {
	return getXGoRegexFromSchema(schema.Value)
}

// getXGoRegexFromSchema reads the x-go-regex extension directly off a Schema
// value, bypassing the SchemaRef wrapper. Useful in code paths that already
// hold the unwrapped Schema (e.g. fieldValidationRuleFromSchema).
func getXGoRegexFromSchema(schema *openapi3.Schema) string {
	if schema == nil {
		return ""
	}
	if len(schema.Extensions) > 0 && schema.Extensions[goRegex] != nil {
		return parseExtensionString(schema.Extensions[goRegex])
	}
	return ""
}

func (generator *Generator) getXGoStringTrimmable(schema *openapi3.SchemaRef) bool {
	if len(schema.Value.Extensions) > 0 && schema.Value.Extensions[goStringTrimmable] != nil {
		return parseExtensionBool(schema.Value.Extensions[goStringTrimmable])
	}

	return false
}

func (generator *Generator) validationFuncFromRules(receiverName string, name string, rules []jen.Code, schema *openapi3.Schema) jen.Code {
	if schema != nil && generator.typee.getXGoSkipValidation(schema) {
		return nil
	}

	block := jen.Return().Id("nil")
	if len(rules) > 0 {
		params := append([]jen.Code{jen.Op("&").Id(receiverName)}, rules...)
		block = jen.Return().Qual("github.com/go-ozzo/ozzo-validation/v4", "ValidateStruct").Call(params...)
	}

	return jen.Func().Params(
		jen.Id(receiverName).Id(name)).Id("Validate").Params().Params(
		jen.Id("error")).Block(block)
}

func (generator *Generator) fieldValidationRuleFromSchema(receiverName string, propertyName string, schema *openapi3.SchemaRef, required bool) jen.Code {
	var fieldRule jen.Code
	v := schema.Value

	if generator.typee.getXGoSkipValidation(v) {
		return fieldRule
	}

	if isSchemaType(v.Type, "string") {
		regex := getXGoRegexFromSchema(v)
		hasLengthRule := v.MaxLength != nil || v.MinLength > 0
		if hasLengthRule || regex != "" {
			var maxLength uint64
			if v.MaxLength != nil {
				maxLength = *v.MaxLength
			}
			params := []jen.Code{jen.Op("&").Id(receiverName).Dot(propertyName)}
			if v.MinLength > 0 && required {
				params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Required"))
			} else if !required {
				// Optional fields: skip every subsequent rule when the value is
				// the zero string. ozzo's IsEmpty short-circuit already covers
				// RuneLength and Match individually, but emitting Skip.When
				// makes the intent obvious in the generated code and matches
				// the pre-existing length-only branch's behavior.
				params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Skip").Dot("When").Call(jen.Id(receiverName).Dot(propertyName).Op("==").Lit("")))
			}
			if hasLengthRule {
				params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "RuneLength").Call(jen.Lit(int(v.MinLength)), jen.Lit(int(maxLength))))
			}
			if regex != "" {
				// Regex belongs in Validate(), not UnmarshalJSON: schema
				// validation is a separate concern from JSON parsing. The
				// useRegex map registered the compiled regexp at codegen time,
				// so we just look up the variable and pass it to validation.Match.
				params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Match").Call(jen.Id(generator.useRegex[regex])))
			}
			fieldRule = jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(params...)
		}
	} else if isSchemaType(v.Type, "integer") || isSchemaType(v.Type, "number") {
		var rules []jen.Code
		if v.Min != nil {
			min := jen.Lit(*v.Min)
			if isSchemaType(v.Type, "integer") {
				min = jen.Lit(int(*v.Min))
			}
			r := jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Min").Call(min)
			if v.ExclusiveMin {
				r.Dot("Exclusive").Call()
			}
			rules = append(rules, r)
		}
		if v.Max != nil {
			max := jen.Lit(*v.Max)
			if isSchemaType(v.Type, "integer") {
				max = jen.Lit(int(*v.Max))
			}
			r := jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Max").Call(max)
			if v.ExclusiveMax {
				r.Dot("Exclusive").Call()
			}
			rules = append(rules, r)
		}
		if len(rules) > 0 {
			params := append([]jen.Code{jen.Op("&").Id(receiverName).Dot(propertyName)}, rules...)
			fieldRule = jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(params...)
		}
	} else if isSchemaType(v.Type, "array") {
		// Arrays: ozzo's validation.Length checks slice length, but it short-
		// circuits on "empty" values — an empty/nil slice always passes
		// Length(min,max) regardless of min. To enforce minItems on a required
		// array we have to pair Length with validation.Required, which is the
		// rule that actually errors on empty input. Mirror the string handling
		// above so a required field with a positive lower bound rejects [].
		if v.MinItems > 0 || v.MaxItems != nil {
			params := []jen.Code{jen.Op("&").Id(receiverName).Dot(propertyName)}
			if v.MinItems > 0 && required {
				params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Required"))
			} else if v.MinItems > 0 {
				// Optional array: an absent field (nil slice) passes, but a
				// present-but-empty [] must still respect minItems. Unlike
				// strings, nil and empty are distinguishable for slices, so
				// skip when nil and require non-empty otherwise.
				params = append(params,
					jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Skip").Dot("When").Call(jen.Id(receiverName).Dot(propertyName).Op("==").Nil()),
					jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Required"))
			}
			var maxItems uint64
			if v.MaxItems != nil {
				maxItems = *v.MaxItems
			}
			params = append(params, jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Length").Call(
				jen.Lit(int(v.MinItems)),
				jen.Lit(int(maxItems)),
			))
			fieldRule = jen.Qual("github.com/go-ozzo/ozzo-validation/v4", "Field").Call(params...)
		}
	}
	return fieldRule
}

func (generator *Generator) componentFromSchema(name string, parentSchema *openapi3.SchemaRef) jen.Code {
	name = generator.normalizer.normalize(name)

	typeDeclaration := jen.Type().Id(name)

	if generator.config.PrioritizeXGoType && generator.typee.hasXGoType(parentSchema.Value) {
		generator.typee.fillGoType(typeDeclaration, "", name, parentSchema, false, true)

		return typeDeclaration
	}

	if len(parentSchema.Value.Properties) == 0 {
		if len(parentSchema.Value.Enum) > 0 {
			generator.typee.fillGoType(typeDeclaration, "", name+"Enum", parentSchema, false, false)
			return typeDeclaration
		}

		//validateFunc := generator.validationFuncFromRules("body", name, nil)
		generator.typee.fillGoType(typeDeclaration, "", name, parentSchema, false, true)

		//return typeDeclaration.Add(jen.Line(), validateFunc)
		return typeDeclaration
	}

	componentStruct := typeDeclaration.Struct(generator.typeProperties(name, parentSchema.Value, false)...)
	helperName := generator.normalizer.decapitalize(name)
	componentHelperStruct := jen.Type().Id(helperName).Struct(generator.typeProperties(helperName, parentSchema.Value, true)...)

	var fieldValidationRules []jen.Code
	var unmarshalNonRequiredAssignments []jen.Code

	// Process non-required properties in deterministic order
	var nonRequiredProps []string
	for propName := range parentSchema.Value.Properties {
		isRequired := slices.Contains(parentSchema.Value.Required, propName)
		if !isRequired {
			nonRequiredProps = append(nonRequiredProps, propName)
		}
	}
	slices.Sort(nonRequiredProps)

	for _, property := range nonRequiredProps {
		schema := parentSchema.Value.Properties[property]
		propertyName := strings.Title(generator.normalizer.normalize(property))

		// Regex validation happens inside Validate() via ozzo's validation.Match
		// (see fieldValidationRuleFromSchema) — UnmarshalJSON only handles
		// presence/absence and JSON well-formedness so a malformed payload and a
		// schema-invalid payload surface through different error paths.
		fvRule := generator.fieldValidationRuleFromSchema("body", propertyName, schema, false)
		if fvRule != nil {
			fieldValidationRules = append(fieldValidationRules, jen.Line().Add(fvRule))
		}

		isTrimmable := generator.getXGoStringTrimmable(schema)
		if isTrimmable {
			unmarshalNonRequiredAssignments = append(unmarshalNonRequiredAssignments,
				jen.Id("body").Dot(propertyName).Op("=").Qual("strings", "TrimSpace").Call(jen.Id("value").Dot(propertyName)).Line())
		} else {
			unmarshalNonRequiredAssignments = append(unmarshalNonRequiredAssignments,
				jen.Id("body").Dot(propertyName).Op("=").Id("value").Dot(propertyName).Line())
		}
	}

	var unmarshalRequiredAssignments []jen.Code

	// Process required properties in deterministic order
	var requiredProps []string
	for _, reqProp := range parentSchema.Value.Required {
		if _, exists := parentSchema.Value.Properties[reqProp]; exists {
			requiredProps = append(requiredProps, reqProp)
		}
	}
	slices.Sort(requiredProps)

	for _, property := range requiredProps {
		schema := parentSchema.Value.Properties[property]
		propertyName := strings.Title(generator.normalizer.normalize(property))

		// Regex validation happens inside Validate() via ozzo's validation.Match
		// (see fieldValidationRuleFromSchema) — UnmarshalJSON only handles
		// presence/absence and JSON well-formedness so a malformed payload and a
		// schema-invalid payload surface through different error paths.
		fvRule := generator.fieldValidationRuleFromSchema("body", propertyName, schema, true)
		if fvRule != nil {
			fieldValidationRules = append(fieldValidationRules, jen.Line().Add(fvRule))
		}

		code := jen.If(jen.Id("value").Dot(propertyName).Op("==").Id("nil")).
			Block(jen.Return().Id(generator.errVar(fmt.Sprintf("field '%s' is required but was null or missing", propertyName), errFileComponents))).
			Line().Line()

		isTrimmable := generator.getXGoStringTrimmable(schema)
		if isTrimmable {
			unmarshalRequiredAssignments = append(unmarshalRequiredAssignments,
				code.Id("body").Dot(propertyName).Op("=").Qual("strings", "TrimSpace").Call(jen.Op("*").Id("value").Dot(propertyName)).Line().Line())
		} else {
			unmarshalRequiredAssignments = append(unmarshalRequiredAssignments,
				code.Id("body").Dot(propertyName).Op("=").Op("*").Id("value").Dot(propertyName).Line().Line())
		}
	}

	unmarshalFunc := jen.Func().Params(
		jen.Id("body").Op("*").Id(name)).Id("UnmarshalJSON").Params(
		jen.Id("data").Index().Id("byte")).Params(
		jen.Id("error")).Block(
		jen.Var().Id("value").Id(helperName),
		jen.If(jen.Id("err").Op(":=").Qual("encoding/json",
			"Unmarshal").Call(jen.Id("data"),
			jen.Op("&").Id("value")),
			jen.Id("err").Op("!=").Id("nil")).Block(
			jen.Return().Id("err")).Line().Line().
			Add(unmarshalNonRequiredAssignments...).Line().Line().
			Add(unmarshalRequiredAssignments...).Line().Line().
			Add(jen.Return().Id("nil"))).Line()

	validateFunc := generator.validationFuncFromRules("body", name, fieldValidationRules, parentSchema.Value)

	return jen.Add(componentHelperStruct).
		Add(jen.Line().Line()).
		Add(componentStruct).
		Add(jen.Line().Line()).
		Add(unmarshalFunc).
		Add(validateFunc)
}

func (generator *Generator) typeProperties(typeName string, schema *openapi3.Schema, pointersForRequired bool) (parameters []jen.Code) {
	// Sort property names to ensure deterministic type property ordering
	var propNames []string
	for propName := range schema.Properties {
		propNames = append(propNames, propName)
	}
	slices.Sort(propNames)

	for _, originName := range propNames {
		schemaRef := schema.Properties[originName]
		name := generator.normalizer.normalize(originName)

		parameter := jen.Id(name)
		if len(schemaRef.Value.Enum) > 0 {
			if schemaRef.Ref != "" {
				name = generator.normalizer.extractNameFromRef(schemaRef.Ref)
			} else {
				name = strings.Title(typeName) + strings.Title(name) + "Enum"
			}
		}

		asPointer := pointersForRequired && slices.Contains(schema.Required, originName)

		generator.typee.fillGoType(parameter, typeName, name, schemaRef, asPointer, false)
		generator.typee.fillJsonTag(parameter, schemaRef, originName)
		parameters = append(parameters, parameter)
	}

	return
}

func (generator *Generator) enums(swagger *openapi3.T) jen.Code {
	var pathsResult []jen.Code

	// Sort paths to ensure deterministic path enum generation order
	var pathNames []string
	for pathName := range swagger.Paths.Map() {
		pathNames = append(pathNames, pathName)
	}
	slices.Sort(pathNames)

	for _, path := range pathNames {
		pathItem := swagger.Paths.Value(path)

		// Sort operations to ensure deterministic operation enum generation order
		var operationMethods []string
		for method := range pathItem.Operations() {
			operationMethods = append(operationMethods, method)
		}
		slices.Sort(operationMethods)

		for _, method := range operationMethods {
			operation := pathItem.Operations()[method]
			var requestBodyResults []jen.Code

			name := generator.normalizer.normalizeOperationName(path, method)

			if operation.RequestBody != nil {
				// Sort content types to ensure deterministic request body enum generation order
				var contentTypes []string
				for contentType := range operation.RequestBody.Value.Content {
					contentTypes = append(contentTypes, contentType)
				}
				slices.Sort(contentTypes)

				for _, contentType := range contentTypes {
					mediaType := operation.RequestBody.Value.Content[contentType]
					schema := mediaType.Schema

					namePrefix := generator.normalizer.normalize(name + generator.normalizer.contentType(contentType))

					if len(schema.Value.Enum) > 0 {
						requestBodyResults = append(requestBodyResults, generator.enumFromSchema(namePrefix+"RequestBodyEnum", schema))
						continue
					}

					var result []jen.Code
					// Sort property names to ensure deterministic enum generation order
					var propNames []string
					for propName, propSchema := range schema.Value.Properties {
						if len(propSchema.Value.Enum) > 0 {
							propNames = append(propNames, propName)
						}
					}
					slices.Sort(propNames)

					for _, propName := range propNames {
						propSchema := schema.Value.Properties[propName]
						enumName := namePrefix + generator.normalizer.normalize(strings.Title(propName)) + "Enum"
						enumName = generator.normalizer.normalize(enumName)
						result = append(result, generator.enumFromSchema(enumName, propSchema))
					}

					if len(result) > 0 {
						requestBodyResults = append(requestBodyResults, jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(result...)...))
					}
				}
			}

			var responseResults []jen.Code
			// Sort response status codes to ensure deterministic response enum generation order
			var statusCodes []string
			for statusCode := range operation.Responses.Map() {
				statusCodes = append(statusCodes, statusCode)
			}
			slices.Sort(statusCodes)

			for _, statusCode := range statusCodes {
				response := operation.Responses.Map()[statusCode]

				// Sort content types to ensure deterministic response content enum generation order
				var contentTypes []string
				for contentType, mediaType := range response.Value.Content {
					if mediaType.Schema.Ref == "" { // only inline schemas
						contentTypes = append(contentTypes, contentType)
					}
				}
				slices.Sort(contentTypes)

				for _, contentType := range contentTypes {
					mediaType := response.Value.Content[contentType]
					schema := mediaType.Schema
					namePrefix := generator.normalizer.normalize(name + generator.normalizer.contentType(contentType))

					if len(schema.Value.Enum) > 0 {
						responseResults = append(responseResults, generator.enumFromSchema(namePrefix+"ResponseBodyEnum", schema))
						continue
					}

					var result []jen.Code
					// Sort property names to ensure deterministic response enum generation order
					var propNames []string
					for propName, propSchema := range schema.Value.Properties {
						if len(propSchema.Value.Enum) > 0 {
							propNames = append(propNames, propName)
						}
					}
					slices.Sort(propNames)

					for _, propName := range propNames {
						propSchema := schema.Value.Properties[propName]
						enumName := namePrefix + generator.normalizer.normalize(strings.Title(propName)) + "Enum"
						enumName = generator.normalizer.normalize(enumName)
						result = append(result, generator.enumFromSchema(enumName, propSchema))
					}

					if len(result) > 0 {
						responseResults = append(responseResults, jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(result...)...))
					}
				}
			}

			// Combine request body and response results for this operation
			var operationResults []jen.Code
			operationResults = append(operationResults, responseResults...)
			operationResults = append(operationResults, requestBodyResults...)
			pathsResult = append(pathsResult, operationResults...)
		}
	}

	var componentsResult []jen.Code

	// Sort schema names to ensure deterministic component enum generation order
	var schemaNames []string
	for schemaName := range swagger.Components.Schemas {
		schemaNames = append(schemaNames, schemaName)
	}
	slices.Sort(schemaNames)

	for _, schemaName := range schemaNames {
		schema := swagger.Components.Schemas[schemaName]
		namePrefix := generator.normalizer.normalize(schemaName)

		if len(schema.Value.Enum) > 0 {
			componentsResult = append(componentsResult, generator.enumFromSchema(namePrefix, schema))
			continue
		}

		var result []jen.Code
		// Sort property names to ensure deterministic component enum generation order
		var propNames []string
		for propName, propSchema := range schema.Value.Properties {
			if len(propSchema.Value.Enum) > 0 {
				propNames = append(propNames, propName)
			}
		}
		slices.Sort(propNames)

		for _, propName := range propNames {
			propSchema := schema.Value.Properties[propName]
			enumName := namePrefix + generator.normalizer.normalize(strings.Title(propName)) + "Enum"
			enumName = generator.normalizer.normalize(enumName)
			result = append(result, generator.enumFromSchema(enumName, propSchema))
		}

		if len(result) > 0 {
			componentsResult = append(componentsResult, jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(result...)...))
		}
	}

	return jen.Null().Add(generator.normalizer.lineAfterEachElement(pathsResult...)...).Add(generator.normalizer.lineAfterEachElement(componentsResult...)...)
}

func (generator *Generator) hooksStruct() jen.Code {
	return jen.Comment("Hooks are observability callbacks fired by the generated routing layer.").
		Line().
		Comment("Any field may be left nil — ensureHooks() at handler-init time fills the").
		Line().
		Comment("nil entries with no-op stubs, so call sites never need to nil-check.").
		Line().
		Comment("").
		Line().
		Comment("Contract: hooks are for observability (logging, metrics, tracing). The").
		Line().
		Comment("runtime owns the response. ResponseBodyMarshalFailed receives an").
		Line().
		Comment("http.ResponseWriter for inspection only — the runtime calls http.Error").
		Line().
		Comment("immediately after the hook returns, so writing to w from the hook will").
		Line().
		Comment("produce a duplicate-WriteHeader warning. Read headers/r.Context() at most.").
		Line().
		Type().Id("Hooks").Struct(
		jen.Id("RequestSecurityParseFailed").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestSecurityParseCompleted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string")),
		jen.Id("RequestSecurityCheckFailed").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestSecurityCheckCompleted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("string")),
		jen.Id("RequestBodyUnmarshalFailed").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestHeaderParseFailed").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestPathParseFailed").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestQueryParseFailed").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestBodyValidationFailed").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestHeaderValidationFailed").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestPathValidationFailed").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestQueryValidationFailed").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("RequestProcessingResult")),
		jen.Id("RequestBodyUnmarshalCompleted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string")),
		jen.Id("RequestHeaderParseCompleted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string")),
		jen.Id("RequestPathParseCompleted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string")),
		jen.Id("RequestQueryParseCompleted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string")),
		jen.Id("RequestParseCompleted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string")),
		jen.Id("RequestProcessingCompleted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string")),
		jen.Id("RequestRedirectStarted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"), jen.Id("string")),
		jen.Id("ResponseBodyMarshalCompleted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string")),
		jen.Id("ResponseBodyWriteCompleted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"), jen.Id("int")),
		jen.Id("ResponseBodyMarshalFailed").Func().Params(
			jen.Qual("net/http", "ResponseWriter"),
			jen.Op("*").Qual("net/http", "Request"),
			jen.Id("string"),
			jen.Id("error")),
		jen.Id("ResponseBodyWriteFailed").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string"),
			jen.Id("int"),
			jen.Id("error")),
		jen.Id("ServiceCompleted").Func().Params(jen.Op("*").Qual("net/http",
			"Request"),
			jen.Id("string")),
	)
}

func (generator *Generator) requestProcessingResultType() jen.Code {
	return jen.Type().Id("requestProcessingResultType").Id("uint8").
		Add(jen.Line(), jen.Line()).
		Add(jen.Const().Defs(
			// ParseSucceed is the zero value so the named-return request
			// already starts in this state without an explicit assignment.
			jen.Id("ParseSucceed").Id("requestProcessingResultType").Op("=").Id("iota"),
			jen.Id("BodyUnmarshalFailed"),
			jen.Id("BodyValidationFailed"),
			jen.Id("HeaderParseFailed"),
			jen.Id("HeaderValidationFailed"),
			jen.Id("QueryParseFailed"),
			jen.Id("QueryValidationFailed"),
			jen.Id("PathParseFailed"),
			jen.Id("PathValidationFailed"),
			jen.Id("SecurityParseFailed"),
			jen.Id("SecurityCheckFailed"),
		)).
		Add(jen.Line(), jen.Line()).
		Add(jen.Type().Id("RequestProcessingResult").Struct(
			jen.Id("error").Id("error"),
			jen.Id("typee").Id("requestProcessingResultType"),
		)).
		Add(jen.Line(), jen.Line()).
		Add(jen.Func().Id("NewRequestProcessingResult").Params(
			jen.Id("t").Id("requestProcessingResultType"),
			jen.Id("err").Id("error")).
			Params(jen.Id("RequestProcessingResult")).Block(
			jen.Return().Id("RequestProcessingResult").Values(jen.Dict{
				jen.Id("typee"): jen.Id("t"),
				jen.Id("error"): jen.Id("err"),
			}))).
		Add(jen.Line(), jen.Line()).
		Add(jen.Func().Params(
			jen.Id("r").Id("RequestProcessingResult")).Id("Type").Params().Params(
			jen.Id("requestProcessingResultType")).Block(
			jen.Return().Id("r").Dot("typee"))).
		Add(jen.Line(), jen.Line()).
		Add(jen.Func().Params(
			jen.Id("r").Id("RequestProcessingResult")).Id("Err").Params().Params(
			jen.Id("error")).Block(
			jen.Return().Id("r").Dot("error"),
		))
}

func (generator *Generator) wrappers(swagger *openapi3.T) jen.Code {
	var results []jen.Code

	// Sort grouped operations by tag for deterministic generation
	groupedOps := generator.groupedOperations(swagger)
	slices.SortFunc(groupedOps, func(a, b groupedOperations) int {
		if a.tag < b.tag {
			return -1
		}
		if a.tag > b.tag {
			return 1
		}
		return 0
	})

	for _, group := range groupedOps {
		tag := generator.normalizer.normalize(group.tag)
		routerName := strings.ToLower(tag) + "Router"

		routes := make([]jen.Code, 0, len(group.operations))
		wrappers := make([]jen.Code, 0, len(group.operations))
		hasSecuritySchemas := false

		for _, op := range group.operations {
			method := generator.normalizer.normalize(strings.Title(strings.ToLower(op.method)))
			baseName := generator.normalizer.normalizeOperationName(op.path, op.method)

			if op.operation.RequestBody == nil || len(op.operation.RequestBody.Value.Content) == 1 {
				routes = append(routes,
					jen.Id("router").Dot("router").Dot(method).Call(jen.Lit(op.path), jen.Id("router").Dot(baseName)))
			} else {
				var routeCodes []jen.Code
				for _, ctEntry := range sortedMapEntries(op.operation.RequestBody.Value.Content) {
					name := baseName + generator.normalizer.contentType(ctEntry.Key)
					routeCodes = append(routeCodes,
						jen.Id("router").Dot("router").Dot(method).Call(jen.Lit(op.path), jen.Id("router").Dot(name)))
				}
				routes = append(routes, jen.Add(generator.normalizer.lineAfterEachElement(routeCodes...)...))
			}

			if op.operation.RequestBody == nil {
				wrappers = append(wrappers,
					generator.wrapper(baseName, baseName+"Request", routerName, method, op.path, op.operation, nil, ""))
			} else if len(op.operation.RequestBody.Value.Content) == 1 {
				entries := sortedMapEntries(op.operation.RequestBody.Value.Content)
				contentType := entries[0].Key
				requestBody := entries[0].Value.Schema
				wrappers = append(wrappers,
					generator.wrapper(baseName, baseName+"Request", routerName, method, op.path, op.operation, requestBody, contentType))
			} else {
				var wrapperCodes []jen.Code
				for _, ctEntry := range sortedMapEntries(op.operation.RequestBody.Value.Content) {
					name := baseName + generator.normalizer.contentType(ctEntry.Key)
					wrapperCodes = append(wrapperCodes,
						generator.wrapper(name, name+"Request", routerName, method, op.path, op.operation, ctEntry.Value.Schema, ctEntry.Key))
				}
				wrappers = append(wrappers, jen.Add(generator.normalizer.lineAfterEachElement(wrapperCodes...)...))
			}

			if op.operation.Security != nil && len(*op.operation.Security) > 0 {
				hasSecuritySchemas = true
			}
		}

		groupCode := jen.Null().
			Add(generator.handler(strings.Title(tag)+"Handler", strings.Title(tag)+"Service", routerName, hasSecuritySchemas, group.operations)).
			Add(jen.Line()).
			Add(generator.router(routerName, strings.Title(tag)+"Service", hasSecuritySchemas, group.operations)).
			Add(jen.Line()).
			Add(jen.Func().Params(jen.Id("router").Op("*").Id(routerName)).Id("mount").Params().Block(routes...)).
			Add(jen.Line(), jen.Line()).
			Add(generator.normalizer.lineAfterEachElement(wrappers...)...).
			Add(jen.Line())
		results = append(results, groupCode)
	}

	// Generate cloneWithBody helper function when PassRawRequest is enabled
	// This function clones the request while preserving the body for both
	// the parsing phase and the original request passed to handlers
	if generator.config.PassRawRequest {
		results = append(results, jen.Func().Id("cloneWithBody").Params(
			jen.Id("r").Op("*").Qual("net/http", "Request"),
		).Params(
			jen.Op("*").Qual("net/http", "Request"),
			jen.Error(),
		).Block(
			jen.List(jen.Id("bodyBytes"), jen.Id("err")).Op(":=").Qual("io", "ReadAll").Call(jen.Id("r").Dot("Body")),
			jen.If(jen.Id("err").Op("!=").Nil()).Block(
				jen.Return(jen.Nil(), jen.Id("err")),
			),
			jen.Id("r").Dot("Body").Dot("Close").Call(),
			jen.Id("r").Dot("Body").Op("=").Qual("io", "NopCloser").Call(
				jen.Qual("bytes", "NewReader").Call(jen.Id("bodyBytes")),
			),
			jen.Id("cloned").Op(":=").Id("r").Dot("Clone").Call(jen.Id("r").Dot("Context").Call()),
			jen.Id("cloned").Dot("Body").Op("=").Qual("io", "NopCloser").Call(
				jen.Qual("bytes", "NewReader").Call(jen.Id("bodyBytes")),
			),
			jen.Return(jen.Id("cloned"), jen.Nil()),
		))
	}

	out := jen.Null().
		Add(generator.hooksStruct()).
		Add(jen.Line(), jen.Line()).
		Add(generator.ensureHooksFunc()).
		Add(jen.Line(), jen.Line()).
		Add(generator.extractSecurityFunc()).
		Add(jen.Line(), jen.Line()).
		Add(generator.requestProcessingResultType()).
		Add(jen.Line(), jen.Line())

	// isNilResponse is only referenced from classic-mode wrappers, so emit it
	// once at the top of routes_gen.go whenever any operation is classic.
	if generator.swaggerHasClassicResponse(swagger) {
		out = out.Add(generator.isNilResponseFunc()).Add(jen.Line(), jen.Line())
	}

	return out.
		Add(generator.normalizer.lineAfterEachElement(results...)...).
		Add(jen.Line(), jen.Line())
}

// swaggerHasClassicResponse reports whether at least one operation uses the
// classic (non-generic) response interface. Used to gate emission of helpers
// that are only referenced from classic-mode wrappers.
func (generator *Generator) swaggerHasClassicResponse(swagger *openapi3.T) bool {
	for _, pathItem := range swagger.Paths.Map() {
		for _, op := range pathItem.Operations() {
			if !generator.useGenerics(op) {
				return true
			}
		}
	}
	return false
}

// isNilResponseFunc emits the one-time isNilResponse helper. The classic-mode
// wrapper uses it instead of `result == nil` because a typed-nil pointer
// returned as the per-op response interface is NOT == nil (the interface holds
// a (type, nil) pair). Reflect drills into the boxed value so both literal nil
// and typed-nil are caught.
func (generator *Generator) isNilResponseFunc() jen.Code {
	const src = `
func isNilResponse(v responseInterface) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Chan, reflect.Func, reflect.Map, reflect.Slice:
		return rv.IsNil()
	}
	return false
}
`
	imports := jen.Var().Defs(jen.Id("_").Op("=").Qual("reflect", "ValueOf"))
	return jen.Null().Add(imports).Line().Line().Op(src)
}

// extractSecurityFunc emits the extractSecurity helper that walks the
// OR-of-AND security matrix. Return shape: passed=true ⇒ one branch
// satisfied; passed=false + checkErr ⇒ handle() rejected (hook already
// fired, caller must NOT fire RequestSecurityParseFailed); passed=false +
// nil checkErr ⇒ no credentials extracted (caller fires it).
func (generator *Generator) extractSecurityFunc() jen.Code {
	const src = `
func extractSecurity(
	r *http.Request,
	hooks *Hooks,
	name string,
	requirements [][]securityProcessor,
	skipCheck bool,
) (results map[SecurityScheme]string, passed bool, checkErr error) {
	if skipCheck {
		// Expose every extractable credential across all OR-branches so the
		// handler can decide. Validation is skipped by contract.
		for _, processors := range requirements {
			for _, processor := range processors {
				_, value, isExtracted := processor.extract(r)
				if !isExtracted {
					continue
				}
				if results == nil {
					results = map[SecurityScheme]string{}
				}
				results[processor.scheme] = value
			}
		}
		return results, true, nil
	}

	for _, processors := range requirements {
		// Per-branch buffer: a partially-extracted branch must not leak its
		// credentials into the final results. Allocated lazily on the first
		// extracted credential so failed branches cost no allocation.
		var branchResults map[SecurityScheme]string
		linkedChecksValid := true
		attemptCheckErr := error(nil)
		for _, processor := range processors {
			schemeName, value, isExtracted := processor.extract(r)
			if !isExtracted {
				linkedChecksValid = false
				break
			}
			if err := processor.handle(r, processor.scheme, schemeName, value); err != nil {
				hooks.RequestSecurityCheckFailed(r, name, string(processor.scheme),
					RequestProcessingResult{error: err, typee: SecurityCheckFailed})
				linkedChecksValid = false
				attemptCheckErr = err
				break
			}
			hooks.RequestSecurityCheckCompleted(r, name, string(processor.scheme))
			if branchResults == nil {
				branchResults = make(map[SecurityScheme]string, len(processors))
			}
			branchResults[processor.scheme] = value
		}
		if linkedChecksValid {
			if len(branchResults) > 0 {
				results = branchResults
			}
			return results, true, nil
		}
		// Preserve the first real handle() error across OR-branches. A later
		// branch that fails only on extract (attemptCheckErr stays nil) must
		// NOT overwrite a real auth-handler error from an earlier branch.
		if checkErr == nil {
			checkErr = attemptCheckErr
		}
	}
	return results, false, checkErr
}
`
	return jen.Op(src)
}

// hookCall emits an unguarded hook invocation; ensureHooks fills nil hooks
// with no-op stubs at init time so call sites need no nil-check.
func (generator *Generator) hookCall(hookName string, args ...jen.Code) jen.Code {
	return jen.Id("router").Dot("hooks").Dot(hookName).Call(args...)
}

// ensureHooksFunc emits ensureHooks, which fills every nil field with a
// no-op stub so the rest of the generated code can call hooks unguarded.
func (generator *Generator) ensureHooksFunc() jen.Code {
	const src = `
func ensureHooks(h *Hooks) *Hooks {
	if h == nil {
		h = &Hooks{}
	}
	if h.RequestSecurityParseFailed == nil {
		h.RequestSecurityParseFailed = func(*http.Request, string, RequestProcessingResult) {}
	}
	if h.RequestSecurityParseCompleted == nil {
		h.RequestSecurityParseCompleted = func(*http.Request, string) {}
	}
	if h.RequestSecurityCheckFailed == nil {
		h.RequestSecurityCheckFailed = func(*http.Request, string, string, RequestProcessingResult) {}
	}
	if h.RequestSecurityCheckCompleted == nil {
		h.RequestSecurityCheckCompleted = func(*http.Request, string, string) {}
	}
	if h.RequestBodyUnmarshalFailed == nil {
		h.RequestBodyUnmarshalFailed = func(*http.Request, string, RequestProcessingResult) {}
	}
	if h.RequestHeaderParseFailed == nil {
		h.RequestHeaderParseFailed = func(*http.Request, string, string, RequestProcessingResult) {}
	}
	if h.RequestPathParseFailed == nil {
		h.RequestPathParseFailed = func(*http.Request, string, string, RequestProcessingResult) {}
	}
	if h.RequestQueryParseFailed == nil {
		h.RequestQueryParseFailed = func(*http.Request, string, string, RequestProcessingResult) {}
	}
	if h.RequestBodyValidationFailed == nil {
		h.RequestBodyValidationFailed = func(*http.Request, string, RequestProcessingResult) {}
	}
	if h.RequestHeaderValidationFailed == nil {
		h.RequestHeaderValidationFailed = func(*http.Request, string, RequestProcessingResult) {}
	}
	if h.RequestPathValidationFailed == nil {
		h.RequestPathValidationFailed = func(*http.Request, string, RequestProcessingResult) {}
	}
	if h.RequestQueryValidationFailed == nil {
		h.RequestQueryValidationFailed = func(*http.Request, string, RequestProcessingResult) {}
	}
	if h.RequestBodyUnmarshalCompleted == nil {
		h.RequestBodyUnmarshalCompleted = func(*http.Request, string) {}
	}
	if h.RequestHeaderParseCompleted == nil {
		h.RequestHeaderParseCompleted = func(*http.Request, string) {}
	}
	if h.RequestPathParseCompleted == nil {
		h.RequestPathParseCompleted = func(*http.Request, string) {}
	}
	if h.RequestQueryParseCompleted == nil {
		h.RequestQueryParseCompleted = func(*http.Request, string) {}
	}
	if h.RequestParseCompleted == nil {
		h.RequestParseCompleted = func(*http.Request, string) {}
	}
	if h.RequestProcessingCompleted == nil {
		h.RequestProcessingCompleted = func(*http.Request, string) {}
	}
	if h.RequestRedirectStarted == nil {
		h.RequestRedirectStarted = func(*http.Request, string, string) {}
	}
	if h.ResponseBodyMarshalCompleted == nil {
		h.ResponseBodyMarshalCompleted = func(*http.Request, string) {}
	}
	if h.ResponseBodyWriteCompleted == nil {
		h.ResponseBodyWriteCompleted = func(*http.Request, string, int) {}
	}
	if h.ResponseBodyMarshalFailed == nil {
		h.ResponseBodyMarshalFailed = func(http.ResponseWriter, *http.Request, string, error) {}
	}
	if h.ResponseBodyWriteFailed == nil {
		h.ResponseBodyWriteFailed = func(*http.Request, string, int, error) {}
	}
	if h.ServiceCompleted == nil {
		h.ServiceCompleted = func(*http.Request, string) {}
	}
	return h
}
`
	return jen.Op(src)
}

func (generator *Generator) wrapper(name string, requestName string, routerName, method string, path string, operation *openapi3.Operation, requestBody *openapi3.SchemaRef, contentType string) jen.Code {
	var funcCode []jen.Code

	funcCode = append(funcCode, jen.Defer().Id("r").Dot("Body").Dot("Close").Call())

	// When PassRawRequest is enabled, clone the request first to preserve body
	// The cloned request is used for parsing, while original request is passed to handler
	if generator.config.PassRawRequest {
		funcCode = append(funcCode,
			jen.List(jen.Id("cloned"), jen.Id("cloneErr")).Op(":=").Id("cloneWithBody").Call(jen.Id("r")),
			jen.If(jen.Id("cloneErr").Op("!=").Nil()).Block(
				jen.Qual("net/http", "Error").Call(
					jen.Id("w"),
					jen.Lit("failed to read request body"),
					jen.Qual("net/http", "StatusInternalServerError"),
				),
				jen.Return(),
			),
		)
	}

	hasContent := false
	if operation.Responses != nil && operation.Responses.Len() > 0 {
		for _, respRef := range operation.Responses.Map() {
			if len(respRef.Value.Content) > 0 {
				hasContent = true
				break
			}
		}
	}

	funcCode = append(funcCode,
		jen.Id("result").Op(":=").
			Id("router").Dot("service").Dot(name).Call(generator.serviceCallParams(name)...),
	)
	// A nil service return would NPE on &result.response / result.inner();
	// surface a 500 with a hook instead. Classic mode goes through
	// isNilResponse because a typed-nil pointer behind the per-op
	// responseInterface is NOT == nil.
	nilResponseErr := generator.errVar("service returned a nil response", errFileRoutes)
	var nilCheckCond jen.Code
	if generator.useGenerics(operation) {
		nilCheckCond = jen.Id("result").Op("==").Nil()
	} else {
		nilCheckCond = jen.Id("isNilResponse").Call(jen.Id("result"))
	}
	funcCode = append(funcCode,
		jen.If(nilCheckCond).Block(
			generator.hookCall("ResponseBodyMarshalFailed",
				jen.Id("w"), jen.Id("r"), jen.Lit(name), jen.Id(nilResponseErr)),
			jen.Qual("net/http", "Error").Call(
				jen.Id("w"),
				jen.Lit("internal server error"),
				jen.Qual("net/http", "StatusInternalServerError"),
			),
			jen.Return(),
		),
	)
	var respArg jen.Code
	if generator.useGenerics(operation) {
		respArg = jen.Op("&").Id("result").Dot("response")
	} else {
		respArg = jen.Id("result").Dot("inner").Call()
	}
	funcCode = append(funcCode,
		jen.Id("respond").Call(
			jen.Id("w"),
			jen.Id("r"),
			jen.Id("router").Dot("hooks"),
			jen.Lit(name),
			respArg,
			jen.Lit(hasContent),
		),
	)

	return jen.Null().
		Add(generator.wrapperRequestParser(name, requestName, routerName, method, path, operation, requestBody, contentType)).
		Add(jen.Line()).
		Add(jen.Func().Params(
			jen.Id("router").Op("*").Id(routerName)).Id(name).Params(
			jen.Id("w").Qual("net/http", "ResponseWriter"),
			jen.Id("r").Op("*").Qual("net/http", "Request")).
			Block(funcCode...)).Line().Line()
}

type groupedOperations struct {
	tag        string
	operations []operationWithPath
}

type operationWithPath struct {
	method    string
	operation *openapi3.Operation
	path      string
}

func (generator *Generator) groupedOperations(swagger *openapi3.T) []groupedOperations {
	// "default" stands in for operations without a tag.
	byTag := map[string][]operationWithPath{}
	for _, pathEntry := range sortedMapEntries(swagger.Paths.Map()) {
		for _, opEntry := range sortedMapEntries(pathEntry.Value.Operations()) {
			operation := opEntry.Value
			tag := "default"
			if len(operation.Tags) > 0 {
				tag = operation.Tags[0]
			}
			byTag[tag] = append(byTag[tag], operationWithPath{
				operation: operation,
				path:      pathEntry.Key,
				method:    opEntry.Key,
			})
		}
	}

	tags := sortedMapKeys(byTag)

	result := make([]groupedOperations, 0, len(tags))
	for _, tag := range tags {
		result = append(result, groupedOperations{tag: tag, operations: byTag[tag]})
	}
	return result
}

func (generator *Generator) handler(name string, serviceName string, routerName string, hasSchemas bool, operations []operationWithPath) jen.Code {
	schemas := jen.Null()
	schemasInterfaceParameter := jen.Null()
	if hasSchemas {
		schemasInterfaceParameter = schemasInterfaceParameter.Id("securitySchemas").Id("SecuritySchemas")

		// Distinct security scheme names across operations, sorted.
		seen := map[string]struct{}{}
		var names []string
		for _, op := range operations {
			if op.operation.Security == nil {
				continue
			}
			for _, req := range *op.operation.Security {
				for k := range req {
					if _, ok := seen[k]; ok {
						continue
					}
					seen[k] = struct{}{}
					names = append(names, k)
				}
			}
		}
		slices.Sort(names)

		declarations := make([]jen.Code, 0, len(names)+1)
		for _, raw := range names {
			n := strings.Title(raw)
			declarations = append(declarations,
				jen.Line().Id("SecurityScheme"+n).Op(":").Values(
					jen.Line().Id("scheme").Op(":").Id("SecurityScheme"+n),
					jen.Line().Id("extract").Op(":").Id("securityExtractorsFuncs").Index(jen.Id("SecurityScheme"+n)),
					jen.Line().Id("handle").Op(":").Id("securitySchemas").Dot("SecurityScheme"+n),
					jen.Line(),
				))
		}

		declarations = append(declarations, jen.Line())

		schemas = schemas.Line().Id("router").Dot("securityHandlers").Op("=").Map(jen.Id("SecurityScheme")).Id("securityProcessor").
			Values(declarations...)
	}

	// Pre-build the per-op [][]securityProcessor at handler-init so the hot
	// path doesn't allocate slice literals on every request.
	var securityReqsInits []jen.Code
	if hasSchemas {
		ops := append([]operationWithPath(nil), operations...)
		slices.SortFunc(ops, func(a, b operationWithPath) int {
			if a.path != b.path {
				return strings.Compare(a.path, b.path)
			}
			return strings.Compare(a.method, b.method)
		})
		for _, op := range ops {
			if op.operation.Security == nil || len(*op.operation.Security) == 0 {
				continue
			}
			opName := generator.normalizer.normalizeOperationName(op.path, op.method)

			var reqValues []jen.Code
			for _, requirement := range *op.operation.Security {
				var handlerLookups []jen.Code
				keys := sortedMapKeys(requirement)
				for _, k := range keys {
					handlerLookups = append(handlerLookups, jen.Id("router").Dot("securityHandlers").Index(jen.Id("SecurityScheme"+strings.Title(k))))
				}
				reqValues = append(reqValues, jen.Values(handlerLookups...))
			}

			securityReqsInits = append(securityReqsInits,
				jen.Line().Id("router").Dot(generator.securityReqsFieldName(opName)).Op("=").
					Index().Index().Id("securityProcessor").Values(reqValues...))
		}
	}

	body := []jen.Code{
		jen.Id("hooks").Op("=").Id("ensureHooks").Call(jen.Id("hooks")),
		jen.Line().Id("router").Op(":=").Op("&").Id(routerName).Values(jen.Id("router").Op(":").Id("r"),
			jen.Id("service").Op(":").Id("impl"), jen.Id("hooks").Op(":").Id("hooks")),
		schemas,
	}
	body = append(body, securityReqsInits...)
	body = append(body,
		jen.Line().Id("router").Dot("mount").Call(),
		jen.Line().Return().Id("router").Dot("router"),
	)

	code := jen.Func().Id(name).
		Params(
			jen.Id("impl").Id(serviceName),
			jen.Id("r").Qual("github.com/go-chi/chi/v5", "Router"),
			jen.Id("hooks").Op("*").Id("Hooks"), schemasInterfaceParameter).
		Params(jen.Qual("net/http", "Handler")).
		Block(body...)

	return code
}

// securityReqsFieldName is the router field holding the pre-built
// [][]securityProcessor for opName.
func (generator *Generator) securityReqsFieldName(opName string) string {
	return generator.normalizer.decapitalize(opName) + "SecurityReqs"
}

func (generator *Generator) router(routerName string, serviceName string, hasSecuritySchemas bool, operations []operationWithPath) jen.Code {
	fields := []jen.Code{
		jen.Id("router").Qual("github.com/go-chi/chi/v5", "Router"),
		jen.Id("service").Id(serviceName),
		jen.Id("hooks").Op("*").Id("Hooks"),
	}
	if hasSecuritySchemas {
		fields = append(fields, jen.Id("securityHandlers").Map(jen.Id("SecurityScheme")).Id("securityProcessor"))
	}

	for _, op := range operations {
		if op.operation.Security == nil || len(*op.operation.Security) == 0 {
			continue
		}
		opName := generator.normalizer.normalizeOperationName(op.path, op.method)
		fields = append(fields, jen.Id(generator.securityReqsFieldName(opName)).Index().Index().Id("securityProcessor"))
	}

	return jen.Type().Id(routerName).Struct(fields...)
}

func (generator *Generator) wrapperRequestParsers(wrapperName string, operation *openapi3.Operation) []jen.Code {
	type group struct {
		in    string
		items []*openapi3.ParameterRef
	}
	byIn := map[string][]*openapi3.ParameterRef{}
	var inOrder []string
	for _, p := range operation.Parameters {
		if _, ok := byIn[p.Value.In]; !ok {
			inOrder = append(inOrder, p.Value.In)
		}
		byIn[p.Value.In] = append(byIn[p.Value.In], p)
	}
	_ = inOrder // first-seen order, unused once we sort
	groups := make([]group, 0, len(byIn))
	for in, items := range byIn {
		groups = append(groups, group{in: in, items: items})
	}
	slices.SortFunc(groups, func(a, b group) int { return strings.Compare(a.in, b.in) })

	var result []jen.Code
	for _, g := range groups {
		for _, parameter := range g.items {
			in := parameter.Value.In
			name := generator.normalizer.normalize(parameter.Value.Name)
			paramName := in + name

			// Flatten allOf-with-$ref into the referenced schema so downstream
			// emitters see the resolved type directly.
			if parameter.Value.Schema.Value.AllOf != nil {
				for _, schema := range parameter.Value.Schema.Value.AllOf {
					if schema.Ref != "" {
						parameter.Value.Schema = &openapi3.SchemaRef{
							Ref:   schema.Ref,
							Value: schema.Value,
						}
						break
					}
				}
			}

			switch {
			case generator.typee.isCustomType(parameter.Value.Schema.Value):
				result = append(result, generator.wrapperCustomType(in, name, paramName, wrapperName, parameter))
			case len(parameter.Value.Schema.Value.Enum) > 0:
				enumType := generator.normalizer.extractNameFromRef(parameter.Value.Schema.Ref)
				result = append(result, generator.wrapperEnum(in, enumType, name, paramName, wrapperName, parameter))
			case isSchemaType(parameter.Value.Schema.Value.Type, "integer"):
				result = append(result, generator.wrapperInteger(in, name, paramName, wrapperName, parameter))
			default:
				result = append(result, generator.wrapperStr(in, name, paramName, wrapperName, parameter))
			}
		}

		inTitle := strings.Title(g.in)
		result = append(result,
			jen.Line().Add(jen.If(jen.Id("err").Op(":=").Id("request").Dot(inTitle).Dot("Validate").Call(),
				jen.Id("err").Op("!=").Id("nil")).
				Block(jen.Id("request").Dot("ProcessingResult").Op("=").Id("RequestProcessingResult").Values(jen.Id("error").Op(":").Id("err"),
					jen.Id("typee").Op(":").Id(inTitle+"ValidationFailed")),
					generator.hookCall("Request"+inTitle+"ValidationFailed",
						jen.Id("r"),
						jen.Lit(wrapperName),
						jen.Id("request").Dot("ProcessingResult")),
					jen.Line().Return())),
			jen.Line().Add(jen.Line()).
				Add(generator.hookCall("Request"+inTitle+"ParseCompleted",
					jen.Id("r"),
					jen.Lit(wrapperName))),
		)
	}

	return generator.normalizer.lineAfterEachElement(result...)
}

func (generator *Generator) wrapRequired(name string, isRequired bool, code jen.Code) jen.Code {
	if !isRequired {
		return jen.If(jen.Id(name).Op("!=").Lit("")).Block(code).Line()
	}

	return code
}

func (generator *Generator) extractRefFromAllOf(schema *openapi3.SchemaRef) string {
	if schema.Value.AllOf == nil {
		return schema.Ref
	}

	for _, s := range schema.Value.AllOf {
		if s.Ref != "" {
			return s.Ref
		}
	}
	return ""
}

func (generator *Generator) wrapperCustomType(in string, name string, paramName string, wrapperName string, parameter *openapi3.ParameterRef) jen.Code {
	result := jen.Null()

	switch in {
	case "header":
		result = result.Add(jen.Id(paramName + "Str").Op(":=").Id("r").Dot("Header").Dot("Get").Call(jen.Lit(parameter.Value.Name)))
	case "query":
		result = result.Add(jen.Id(paramName + "Str").Op(":=").Id("query").Dot("Get").Call(jen.Lit(parameter.Value.Name)))
	case "path":
		result = result.Add(jen.Id(paramName+"Str").Op(":=").Id("chi").Dot("URLParam").Call(jen.Id("r"), jen.Lit(parameter.Value.Name)))
	default:
		panic("unsupported " + in + " type")
	}

	result = result.Add(jen.Line())

	parseFailed := []jen.Code{
		jen.Id("request").Dot("ProcessingResult").Op("=").Id("RequestProcessingResult").Values(jen.Id("error").Op(":").Id("err"),
			jen.Id("typee").Op(":").Id(strings.Title(in)+"ParseFailed")),
		generator.hookCall("Request"+strings.Title(in)+"ParseFailed",
			jen.Id("r"),
			jen.Lit(wrapperName),
			jen.Lit(parameter.Value.Name),
			jen.Id("request").Dot("ProcessingResult")),
		jen.Line().Return(),
	}

	if pkg, parse, ok := generator.typee.getXGoTypeStringParse(parameter.Value.Schema.Value); ok {
		parameterCode := jen.Null().
			Add(jen.List(jen.Id(paramName), jen.Id("err")).Op(":=").Qual(pkg, parse).Call(jen.Id(paramName+"Str"))).
			Add(jen.Line()).
			Add(jen.If(jen.Id("err").Op("!=").Id("nil")).Block(parseFailed...)).
			Add(jen.Line(), jen.Line()).
			Add(jen.Id("request").Dot(strings.Title(in)).Dot(name).Op("=").Id(paramName))

		result.Add(generator.wrapRequired(paramName+"Str", parameter.Value.Required, parameterCode))
	} else {
		ref := generator.extractRefFromAllOf(parameter.Value.Schema)
		if ref != "" {
			parameter.Value.Schema.Ref = ref
		}

		switch parameter.Value.Schema.Value.Format {
		case "uuid":
			parameterCode := jen.Null().
				Add(jen.List(jen.Id(paramName), jen.Id("err")).Op(":=").Id("uuid").Dot("Parse").Call(jen.Id(paramName+"Str"))).
				Add(jen.Line()).
				Add(jen.If(jen.Id("err").Op("!=").Id("nil")).Block(parseFailed...)).
				Add(jen.Line(), jen.Line()).
				Add(jen.Id("request").Dot(strings.Title(in)).Dot(name).Op("=").Id(paramName))

			result.Add(generator.wrapRequired(paramName+"Str", parameter.Value.Required, parameterCode))
			break
		case "iso4217-currency-code":
			parameterCode := jen.Null().
				Add(jen.List(jen.Id(paramName), jen.Id("err")).Op(":=").Qual("github.com/mikekonan/go-types/v2/currency", "ByCodeStrErr").Call(jen.Id(paramName+"Str"))).
				Add(jen.Line()).
				Add(jen.If(jen.Id("err").Op("!=").Id("nil")).Block(parseFailed...)).
				Add(jen.Line(), jen.Line()).
				Add(jen.Id("request").Dot(strings.Title(in)).Dot(name).Op("=").Id(paramName).Dot("Code").Call())

			result.Add(generator.wrapRequired(paramName+"Str", parameter.Value.Required, parameterCode))
			break
		case "iso3166-alpha-2":
			parameterCode := jen.Null().
				Add(jen.List(jen.Id(paramName), jen.Id("err")).Op(":=").Qual("github.com/mikekonan/go-types/v2/country", "ByAlpha2CodeStrErr").Call(jen.Id(paramName+"Str"))).
				Add(jen.Line()).
				Add(jen.If(jen.Id("err").Op("!=").Id("nil")).Block(parseFailed...)).
				Add(jen.Line(), jen.Line()).
				Add(jen.Id("request").Dot(strings.Title(in)).Dot(name).Op("=").Id(paramName).Dot("Alpha2Code").Call())

			result.Add(generator.wrapRequired(paramName+"Str", parameter.Value.Required, parameterCode))
			break
		default:
		}
	}

	return result.Line()
}

func (generator *Generator) wrapperEnum(in string, enumType string, name string, paramName string, wrapperName string, parameter *openapi3.ParameterRef) jen.Code {
	result := jen.Null()

	switch in {
	case "header":
		result = result.Add(jen.Id(paramName).Op(":=").Qual(generator.config.ComponentsPackage, enumType).Call(jen.Id("r").Dot("Header").Dot("Get").Call(jen.Lit(parameter.Value.Name))))
	case "query":
		result = result.Add(jen.Id(paramName).Op(":=").Qual(generator.config.ComponentsPackage, enumType).Call(jen.Id("query").Dot("Get").Call(jen.Lit(parameter.Value.Name))))
	case "path":
		result = result.Add(jen.Id(paramName).Op(":=").Qual(generator.config.ComponentsPackage, enumType).Call(jen.Id("chi").Dot("URLParam").Call(jen.Id("r"), jen.Lit(parameter.Value.Name))))
	default:
		panic("unsupported " + in + " type")
	}

	result = result.
		Add(jen.Line()).
		Add(jen.If(jen.Id("err").Op(":=").Id(paramName).Dot("Check").Call(),
			jen.Id("err").Op("!=").Id("nil")).Block(
			jen.Id("request").Dot("ProcessingResult").Op("=").Id("RequestProcessingResult").Values(jen.Id("error").Op(":").Id("err"),
				jen.Id("typee").Op(":").Id(strings.Title(in)+"ParseFailed")),
			generator.hookCall("Request"+strings.Title(in)+"ParseFailed",
				jen.Id("r"),
				jen.Lit(wrapperName),
				jen.Lit(parameter.Value.Name),
				jen.Id("request").Dot("ProcessingResult")),
			jen.Line().Return())).
		Add(jen.Line(), jen.Line()).
		Add(jen.Id("request").Dot(strings.Title(parameter.Value.In)).Dot(name).Op("=").Id(paramName)).
		Add(jen.Line())

	return jen.Null().Add(generator.wrapRequired(paramName, parameter.Value.Required, result)).Line()
}

func (generator *Generator) wrapperStr(in string, name string, paramName string, wrapperName string, parameter *openapi3.ParameterRef) jen.Code {
	result := jen.Null()

	switch in {
	case "header":
		result = result.Add(jen.Id(paramName).Op(":=").Id("r").Dot("Header").Dot("Get").Call(jen.Lit(parameter.Value.Name)))
	case "query":
		result = result.Add(jen.Id(paramName).Op(":=").Id("query").Dot("Get").Call(jen.Lit(parameter.Value.Name)))
	case "path":
		result = result.Add(jen.Id(paramName).Op(":=").Id("chi").Dot("URLParam").Call(jen.Id("r"), jen.Lit(parameter.Value.Name)))
	default:
		panic("unsupported " + in + " type")
	}

	if parameter.Value.Required {
		emptyErr := generator.errVar(fmt.Sprintf("%s is empty", parameter.Value.Name), errFileRoutes)
		result = result.
			Add(jen.Line()).
			Add(jen.If(jen.Id(paramName).Op("==").Lit("")).Block(
				jen.Id("err").Op(":=").Id(emptyErr).Line(),
				jen.Id("request").Dot("ProcessingResult").Op("=").Id("RequestProcessingResult").Values(jen.Id("error").Op(":").Id("err"),
					jen.Id("typee").Op(":").Id(strings.Title(in)+"ParseFailed")),
				generator.hookCall("Request"+strings.Title(in)+"ParseFailed",
					jen.Id("r"),
					jen.Lit(wrapperName),
					jen.Lit(parameter.Value.Name),
					jen.Id("request").Dot("ProcessingResult")),
				jen.Line().Return())).
			Add(jen.Line())
	}

	regex := generator.getXGoRegex(parameter.Value.Schema)
	if regex != "" {
		regexVarName := generator.useRegex[regex]
		regexErr := generator.errVar(fmt.Sprintf("%s not matched by the '%s' regex", parameter.Value.Name, regex), errFileRoutes)

		result = result.Line().If(jen.Op("!").Id(regexVarName).Dot("MatchString").Call(jen.Id(paramName))).Block(
			jen.Id("err").Op(":=").Id(regexErr),
			jen.Line(),
			jen.Id("request").Dot("ProcessingResult").Op("=").Id("RequestProcessingResult").Values(jen.Id("error").Op(":").Id("err"),
				jen.Id("typee").Op(":").Id(fmt.Sprintf("%sParseFailed", strings.Title(in)))),
			generator.hookCall("Request"+strings.Title(in)+"ParseFailed",
				jen.Id("r"),
				jen.Lit(wrapperName),
				jen.Lit(parameter.Value.Name),
				jen.Id("request").Dot("ProcessingResult")),
			jen.Line(),
			jen.Return()).
			Line()
	}

	result = result.
		Line().
		Add(jen.Id("request").Dot(strings.Title(parameter.Value.In)).Dot(name).Op("=").Id(paramName)).
		Line()

	return result
}

func (generator *Generator) wrapperInteger(in string, name string, paramName string, wrapperName string, parameter *openapi3.ParameterRef) jen.Code {
	result := jen.Null()

	switch in {
	case "header":
		result = result.Add(jen.Id(paramName).Op(":=").Id("r").Dot("Header").Dot("Get").Call(jen.Lit(parameter.Value.Name)))
	case "query":
		result = result.Add(jen.Id(paramName).Op(":=").Id("query").Dot("Get").Call(jen.Lit(parameter.Value.Name)))
	case "path":
		result = result.Add(jen.Id(paramName).Op(":=").Id("chi").Dot("URLParam").Call(jen.Id("r"), jen.Lit(parameter.Value.Name)))
	default:
		panic("unsupported " + in + " type")
	}

	if parameter.Value.Required {
		emptyErr := generator.errVar(fmt.Sprintf("%s is empty", parameter.Value.Name), errFileRoutes)
		result = result.
			Add(jen.Line()).
			Add(jen.If(jen.Id(paramName).Op("==").Lit("")).Block(
				jen.Id("err").Op(":=").Id(emptyErr).Line(),
				jen.Id("request").Dot("ProcessingResult").Op("=").Id("RequestProcessingResult").Values(jen.Id("error").Op(":").Id("err"),
					jen.Id("typee").Op(":").Id(strings.Title(in)+"ParseFailed")),
				generator.hookCall("Request"+strings.Title(in)+"ParseFailed",
					jen.Id("r"),
					jen.Lit(wrapperName),
					jen.Lit(parameter.Value.Name),
					jen.Id("request").Dot("ProcessingResult")),
				jen.Line().Return())).
			Add(jen.Line())
	}

	return result.
		Add(jen.Line()).
		Add(jen.Id("request").Dot(strings.Title(parameter.Value.In)).Dot(name).Op("=").Qual("github.com/spf13/cast", "ToInt").Call(jen.Id(paramName))).
		Add(jen.Line())
}

func (generator *Generator) wrapperBody(method string, path string, contentType string, wrapperName string, operation *openapi3.Operation, body *openapi3.SchemaRef) jen.Code {
	result := jen.Null()

	if operation.RequestBody == nil {
		return result
	}

	name := generator.normalizer.extractNameFromRef(body.Ref)

	if name == "" {
		name = generator.normalizer.normalizeOperationName(path, method) + generator.normalizer.contentType(contentType) + "RequestBody"
	}

	result = result.
		Add(jen.Var().Defs(
			jen.Id("body").Qual(generator.config.ComponentsPackage, name),
			jen.Id("decodeErr").Error(),
		)).
		Add(jen.Line()).
		Add(func() *jen.Statement {
			switch contentType {
			case "application/xml":
				return jen.Id("decodeErr").Op("=").Qual("encoding/xml", "NewDecoder").Call(jen.Id("r").Dot("Body")).Dot("Decode").Call(jen.Op("&").Id("body"))

			case "application/octet-stream":
				// io.ReadAll returns ([]byte, error). Surface a read error as
				// the body-unmarshal error (was silently swallowed before) and
				// convert the bytes through the body type so this works for
				// both `type X = []byte` aliases and named `type X []byte`.
				return jen.Add(
					jen.List(jen.Id("bodyBytes"), jen.Id("readErr")).Op(":=").Qual("io", "ReadAll").Call(jen.Id("r").Dot("Body")),
					jen.Line(),
					jen.If(jen.Id("readErr").Op("!=").Nil()).Block(
						jen.Id("decodeErr").Op("=").Id("readErr"),
					).Else().Block(
						jen.Id("body").Op("=").Qual(generator.config.ComponentsPackage, name).Call(jen.Id("bodyBytes")),
					),
				)
			default:
				return jen.Id("decodeErr").Op("=").Qual("encoding/json", "NewDecoder").Call(jen.Id("r").Dot("Body")).Dot("Decode").Call(jen.Op("&").Id("body"))
			}
		}()).
		Add(jen.Line()).
		Add(jen.If(jen.Id("decodeErr").Op("!=").Id("nil")).Block(
			jen.Id("request").Dot("ProcessingResult").Op("=").Id("RequestProcessingResult").Values(jen.Id("error").Op(":").Id("decodeErr"),
				jen.Id("typee").Op(":").Id("BodyUnmarshalFailed")),
			generator.hookCall("RequestBodyUnmarshalFailed",
				jen.Id("r"),
				jen.Lit(wrapperName),
				jen.Id("request").Dot("ProcessingResult")),
			jen.Line().Return())).
		Add(jen.Line(), jen.Line()).
		Add(jen.Id("request").Dot("Body").Op("=").Id("body")).
		Add(jen.Line(), jen.Line()).
		Add(generator.hookCall("RequestBodyUnmarshalCompleted",
			jen.Id("r"),
			jen.Lit(wrapperName))).
		Add(jen.Line())

	if contentType != "application/octet-stream" && !generator.typee.getXGoSkipValidation(body.Value) {
		result = result.Add(jen.Line()).Add(jen.If(jen.Id("err").Op(":=").Id("request").Dot("Body").Dot("Validate").Call(),
			jen.Id("err").Op("!=").Id("nil")).
			Block(jen.Id("request").Dot("ProcessingResult").Op("=").Id("RequestProcessingResult").Values(jen.Id("error").Op(":").Id("err"),
				jen.Id("typee").Op(":").Id("BodyValidationFailed")),
				generator.hookCall("RequestBodyValidationFailed",
					jen.Id("r"),
					jen.Lit(wrapperName),
					jen.Id("request").Dot("ProcessingResult")),
				jen.Line().Return()))
	}

	return result.Add(jen.Line())
}
func (generator *Generator) wrapperSecurity(name string, operation *openapi3.Operation) jen.Code {
	hasSecuritySchemas := operation.Security != nil && len(*operation.Security) > 0
	if !hasSecuritySchemas {
		return jen.Null()
	}

	skipSecurityCheck := generator.typee.getXGoSkipSecurityCheck(operation)
	skipCheckLit := jen.Lit(skipSecurityCheck)
	securityFailedErr := generator.errVar("failed passing security checks", errFileRoutes)

	// checkErr from extractSecurity carries a handle() failure; the hook for
	// it was already fired inside extractSecurity, so we only fire
	// RequestSecurityParseFailed on the no-credentials branch.
	return jen.Null().
		Add(jen.Line()).
		Add(jen.List(jen.Id("results"), jen.Id("passed"), jen.Id("checkErr")).Op(":=").
			Id("extractSecurity").Call(
			jen.Id("r"),
			jen.Id("router").Dot("hooks"),
			jen.Lit(name),
			jen.Id("router").Dot(generator.securityReqsFieldName(name)),
			skipCheckLit,
		)).
		Add(jen.Line()).
		Add(jen.Id("request").Dot("SecurityCheckResults").Op("=").Id("results")).
		Add(jen.Line()).
		Add(jen.If(jen.Op("!").Id("passed")).Block(
			jen.If(jen.Id("checkErr").Op("!=").Nil()).Block(
				jen.Id("request").Dot("ProcessingResult").Op("=").Id("RequestProcessingResult").Values(
					jen.Id("error").Op(":").Id("checkErr"),
					jen.Id("typee").Op(":").Id("SecurityCheckFailed"),
				),
				jen.Return(),
			),
			jen.Id("request").Dot("ProcessingResult").Op("=").Id("RequestProcessingResult").Values(
				jen.Id("error").Op(":").Id(securityFailedErr),
				jen.Id("typee").Op(":").Id("SecurityParseFailed"),
			),
			generator.hookCall("RequestSecurityParseFailed",
				jen.Id("r"),
				jen.Lit(name),
				jen.Id("request").Dot("ProcessingResult"),
			),
			jen.Return(),
		)).
		Add(jen.Line()).
		Add(generator.hookCall("RequestSecurityParseCompleted", jen.Id("r"), jen.Lit(name))).
		Add(jen.Line())
}
func (generator *Generator) wrapperRequestParser(name string, requestName string, routerName, method string, path string, operation *openapi3.Operation, requestBody *openapi3.SchemaRef, contentType string) jen.Code {
	var funcCode []jen.Code

	// Cache r.URL.Query() once — url.ParseQuery allocates a map + does per-key
	// parsing, so calling it per query param adds up on hot paths.
	hasQueryParam := false
	for _, p := range operation.Parameters {
		if p.Value.In == "query" {
			hasQueryParam = true
			break
		}
	}
	if hasQueryParam {
		funcCode = append(funcCode, jen.Id("query").Op(":=").Id("r").Dot("URL").Dot("Query").Call())
	}

	security := generator.wrapperSecurity(name, operation)
	parsers := generator.wrapperRequestParsers(name, operation)
	body := generator.wrapperBody(method, path, contentType, name, operation, requestBody)

	funcCode = append(funcCode, security)
	funcCode = append(funcCode, parsers...)
	funcCode = append(funcCode, body)

	// Skip the leading blank line before RequestParseCompleted when the
	// function body would otherwise open with an empty line.
	hasPriorContent := hasQueryParam || !isEmptyCode(security) || len(parsers) > 0 || !isEmptyCode(body)
	hookStmt := generator.hookCall("RequestParseCompleted", jen.Id("r"), jen.Lit(name))
	if hasPriorContent {
		funcCode = append(funcCode, jen.Line().Add(hookStmt))
	} else {
		funcCode = append(funcCode, hookStmt)
	}
	funcCode = append(funcCode, jen.Line().Return())

	return jen.Func().Params(
		jen.Id("router").Op("*").Id(routerName)).Id("parse" + name + "Request").
		Params(jen.Id("r").Op("*").Qual("net/http", "Request")).
		Params(jen.Id("request").Id(requestName)).
		Block(funcCode...).
		Line()
}

func (generator *Generator) useGenerics(op *openapi3.Operation) bool {
	if op == nil {
		return false
	}
	if v, ok := op.Extensions[goGenerics]; ok && v != nil {
		return parseExtensionBool(v)
	}
	return generator.config.DefaultGenerics
}

func (generator *Generator) swaggerUsesGenerics(swagger *openapi3.T) bool {
	for _, pathItem := range swagger.Paths.Map() {
		for _, op := range pathItem.Operations() {
			if generator.useGenerics(op) {
				return true
			}
		}
	}
	return false
}

// genericResponseBodyTypeName picks the body type for the generic signature:
// the first 2xx application/json schema, or "any" if none. Inline schemas
// MUST share the name produced by inlineResponseBodyTypeName so the
// signature references a type that components() actually emits.
func (generator *Generator) genericResponseBodyTypeName(op *openapi3.Operation, opName string) string {
	if op.Responses == nil {
		return "any"
	}
	preferred := []string{"200", "201", "202", "204"}
	statusOrder := append([]string{}, preferred...)
	var extras []string
	for status := range op.Responses.Map() {
		if !slices.Contains(preferred, status) && strings.HasPrefix(status, "2") {
			extras = append(extras, status)
		}
	}
	slices.Sort(extras)
	statusOrder = append(statusOrder, extras...)
	for _, status := range statusOrder {
		respRef := op.Responses.Map()[status]
		if respRef == nil || respRef.Value == nil {
			continue
		}
		mt, ok := respRef.Value.Content["application/json"]
		if !ok || mt.Schema == nil {
			continue
		}
		if mt.Schema.Ref != "" {
			return generator.normalizer.extractNameFromRef(mt.Schema.Ref)
		}
		return generator.inlineResponseBodyTypeName(opName, status, "application/json")
	}
	return "any"
}

func (generator *Generator) requestResponseBuilders(swagger *openapi3.T) jen.Code {
	result := []jen.Code{
		generator.responseStruct(),
		generator.respondFunc(),
		generator.handlersTypes(swagger),
		generator.builders(swagger),
		generator.handlersInterfaces(swagger),
		generator.requestParameters(swagger.Paths.Map()),
	}

	if generator.swaggerUsesGenerics(swagger) {
		result = append(result, generator.genericResponseTypes())
	}

	result = generator.normalizer.doubleLineAfterEachElement(result...)

	return jen.Null().Add(result...)
}

// respondFunc emits the respond helper. It takes *response directly; the
// wrapper unwraps from generic *Response[B] or the classic per-op interface.
func (generator *Generator) respondFunc() jen.Code {
	const src = `
func respond(w http.ResponseWriter, r *http.Request, hooks *Hooks, name string, resp *response, hasContent bool) {
	for header, value := range resp.headers {
		w.Header().Set(header, value)
	}

	cookies := resp.cookies
	for i := range cookies {
		http.SetCookie(w, &cookies[i])
	}

	if url := resp.redirectURL; url != "" {
		switch resp.statusCode {
		case 301, 302, 303, 307, 308:
			hooks.RequestRedirectStarted(r, name, url)
			http.Redirect(w, r, url, resp.statusCode)
			hooks.ServiceCompleted(r, name)
			return
		}
	}

	hooks.RequestProcessingCompleted(r, name)

	if !hasContent {
		w.WriteHeader(resp.statusCode)
		hooks.ServiceCompleted(r, name)
		return
	}

	// Marshal BEFORE writing the status code so a serialization failure can
	// still surface as a clean 500 instead of a 2xx with a truncated body —
	// once WriteHeader fires, headers/status are committed to the wire and
	// we can no longer signal the error to the client.
	var body []byte
	if resp.body != nil {
		var err error
		switch resp.contentType {
		case "application/xml":
			body, err = xml.Marshal(resp.body)
		case "application/octet-stream":
			// Fast path for []byte and its aliases (e.g. type X = []byte).
			// Fall back to reflect for named types whose underlying type is
			// []byte (type X []byte) — the plain type assertion above would
			// fail those since Go checks type identity, not underlying type.
			if b, ok := (resp.body).([]byte); ok {
				body = b
			} else {
				rv := reflect.ValueOf(resp.body)
				if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
					body = rv.Bytes()
				} else {
					err = errors.New("body is not []byte")
				}
			}
		case "text/html":
			body = []byte(fmt.Sprint(resp.body))
		case "application/json":
			fallthrough
		default:
			body, err = json.Marshal(resp.body)
		}
		if err != nil {
			hooks.ResponseBodyMarshalFailed(w, r, name, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		hooks.ResponseBodyMarshalCompleted(r, name)
	} else if len(resp.bodyRaw) > 0 {
		body = resp.bodyRaw
	}

	if ct := resp.contentType; ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.statusCode)

	if len(body) > 0 {
		count, err := w.Write(body)
		if err != nil {
			hooks.ResponseBodyWriteFailed(r, name, count, err)
			return
		}
		hooks.ResponseBodyWriteCompleted(r, name, count)
	}

	hooks.ServiceCompleted(r, name)
}
`
	// Raw-string src bypasses jennifer's import tracking; declare the stdlib
	// uses below so the generated file keeps these imports.
	imports := jen.Var().Defs(
		jen.Id("_").Op("=").Qual("encoding/xml", "Marshal"),
		jen.Id("_").Op("=").Qual("encoding/json", "Marshal"),
		jen.Id("_").Op("=").Qual("errors", "New"),
		jen.Id("_").Op("=").Qual("fmt", "Sprint"),
		jen.Id("_").Op("=").Qual("reflect", "ValueOf"),
	)
	return jen.Null().Add(imports).Line().Line().Op(src)
}

type operationResponse struct {
	ContentTypeBodyNameMap map[string]string
	Headers                map[string]*openapi3.HeaderRef
	SetCookie              bool
	StatusCode             string
}

type operationStruct struct {
	Tag                   string
	Name                  string
	RequestName           string
	ResponseName          string
	Responses             []operationResponse
	InterfaceResponseName string
	PrivateName           string
	UseGenerics           bool
}

func (generator *Generator) builders(swagger *openapi3.T) (result jen.Code) {
	var builders []jen.Code

	// Sort paths to ensure deterministic builders generation order
	var pathNames []string
	for pathName := range swagger.Paths.Map() {
		pathNames = append(pathNames, pathName)
	}
	slices.Sort(pathNames)

	for _, pathName := range pathNames {
		pathItem := swagger.Paths.Value(pathName)
		var operationStructs []operationStruct

		// Sort operations to ensure deterministic operation builders generation order
		var operationMethods []string
		for method := range pathItem.Operations() {
			operationMethods = append(operationMethods, method)
		}
		slices.Sort(operationMethods)

		for _, method := range operationMethods {
			operation := pathItem.Operations()[method]
			name := generator.normalizer.normalizeOperationName(pathName, method)
			var operationResponses []operationResponse

			// Sort response status codes to ensure deterministic response processing order
			var statusCodes []string
			for statusCode := range operation.Responses.Map() {
				statusCodes = append(statusCodes, statusCode)
			}
			slices.Sort(statusCodes)

			for _, statusCode := range statusCodes {
				responseRef := operation.Responses.Map()[statusCode]
				var response operationResponse
				response.ContentTypeBodyNameMap = map[string]string{}

				headers := map[string]*openapi3.HeaderRef{}
				// Sort header names to ensure deterministic ordering
				var headerNames []string
				for k := range responseRef.Value.Headers {
					headerNames = append(headerNames, k)
				}
				slices.Sort(headerNames)

				for _, k := range headerNames {
					v := responseRef.Value.Headers[k]
					if strings.ToLower(k) == "set-cookie" {
						response.SetCookie = true
						continue
					}

					if strings.ToLower(k) == "content-encoding" {
						continue
					}

					headers[k] = v
				}

				response.Headers = headers

				// Sort content types to ensure deterministic content type processing order
				var contentTypes []string
				for contentType := range responseRef.Value.Content {
					contentTypes = append(contentTypes, contentType)
				}
				slices.Sort(contentTypes)

				for _, contentType := range contentTypes {
					mediaType := responseRef.Value.Content[contentType]
					var structName string
					if "" == mediaType.Schema.Ref {
						structName = generator.inlineResponseBodyTypeName(name, statusCode, contentType)
					} else {
						structName = generator.normalizer.extractNameFromRef(mediaType.Schema.Ref)
					}
					response.ContentTypeBodyNameMap[contentType] = structName
				}

				response.StatusCode = statusCode
				operationResponses = append(operationResponses, response)
			}

			var tag string
			if len(operation.Tags) > 0 {
				tag = operation.Tags[0]
			} else {
				tag = "default"
			}

			operationStructs = append(operationStructs, operationStruct{
				Tag:                   tag,
				Name:                  name,
				PrivateName:           generator.normalizer.decapitalize(name),
				RequestName:           name + "Request",
				InterfaceResponseName: name + "Response",
				ResponseName:          generator.normalizer.decapitalize(name + "Response"),
				Responses:             operationResponses,
				UseGenerics:           generator.useGenerics(operation),
			})
		}

		for _, operationStruct := range operationStructs {
			if operationStruct.UseGenerics {
				continue
			}
			builders = append(builders, generator.responseBuilders(operationStruct))
		}
	}

	return jen.Null().Add(builders...)
}

func (generator *Generator) handlersTypes(swagger *openapi3.T) jen.Code {
	var result []jen.Code

	// Sort paths to ensure deterministic handlers types generation order
	var pathNames []string
	for pathName := range swagger.Paths.Map() {
		pathNames = append(pathNames, pathName)
	}
	slices.Sort(pathNames)

	for _, pathName := range pathNames {
		pathItem := swagger.Paths.Value(pathName)
		var pathResult []jen.Code

		// Sort operations to ensure deterministic operation handlers types generation order
		var operationMethods []string
		for method := range pathItem.Operations() {
			operationMethods = append(operationMethods, method)
		}
		slices.Sort(operationMethods)

		for _, method := range operationMethods {
			name := generator.normalizer.normalizeOperationName(pathName, method)
			if generator.useGenerics(pathItem.Operations()[method]) {
				continue
			}
			pathResult = append(pathResult, jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(generator.responseType(name))...))
		}

		pathResult = generator.normalizer.doubleLineAfterEachElement(pathResult...)
		result = append(result, jen.Null().Add(pathResult...))
	}

	result = generator.normalizer.doubleLineAfterEachElement(result...)
	return jen.Null().Add(result...)
}

// interfaceMethodParams generates the parameter list for interface methods based on configuration
func (generator *Generator) interfaceMethodParams(requestTypeName string) []jen.Code {
	params := []jen.Code{
		jen.Qual("context", "Context"),
		jen.Id(requestTypeName),
	}

	if generator.config.PassRawRequest {
		params = append(params, jen.Op("*").Qual("net/http", "Request"))
	}

	return params
}

// serviceCallParams generates the parameter list for service method calls based on configuration
// When PassRawRequest is enabled, parsing is done on a cloned request to preserve the body
// in the original request that gets passed to the handler
func (generator *Generator) serviceCallParams(name string) []jen.Code {
	var params []jen.Code

	if generator.config.PassRawRequest {
		// Use cloned request for parsing to preserve body in original request
		params = []jen.Code{
			jen.Id("r").Dot("Context").Call(),
			jen.Id("router").Dot("parse" + name + "Request").Call(jen.Id("cloned")),
			jen.Id("r"),
		}
	} else {
		params = []jen.Code{
			jen.Id("r").Dot("Context").Call(),
			jen.Id("router").Dot("parse" + name + "Request").Call(jen.Id("r")),
		}
	}

	return params
}

func (generator *Generator) serviceReturnType(op *openapi3.Operation, name string) jen.Code {
	if !generator.useGenerics(op) {
		return jen.Id(name + "Response")
	}
	body := generator.genericResponseBodyTypeName(op, name)
	if body == "any" {
		return jen.Op("*").Id("Response").Index(jen.Any())
	}
	// Qual handles cross-package import when -componentsPackage differs.
	return jen.Op("*").Id("Response").Index(jen.Qual(generator.config.ComponentsPackage, body))
}

func (generator *Generator) handlersInterfaces(swagger *openapi3.T) jen.Code {
	// Ops without tags fall under "Default".
	methodsByTag := map[string][]jen.Code{}
	for _, pathEntry := range sortedMapEntries(swagger.Paths.Map()) {
		path := pathEntry.Key
		for _, opEntry := range sortedMapEntries(pathEntry.Value.Operations()) {
			operation := opEntry.Value
			tag := generator.normalizer.normalize("Default")
			if len(operation.Tags) > 0 {
				tag = generator.normalizer.normalize(operation.Tags[0])
			}

			name := generator.normalizer.normalizeOperationName(path, opEntry.Key)
			returnType := generator.serviceReturnType(operation, name)

			if operation.RequestBody == nil || len(operation.RequestBody.Value.Content) == 1 {
				methodsByTag[tag] = append(methodsByTag[tag],
					jen.Id(name).Params(generator.interfaceMethodParams(name+"Request")...).Params(returnType))
				continue
			}

			for _, ctEntry := range sortedMapEntries(operation.RequestBody.Value.Content) {
				contentTypedName := name + generator.normalizer.contentType(ctEntry.Key)
				methodsByTag[tag] = append(methodsByTag[tag],
					jen.Id(contentTypedName).Params(generator.interfaceMethodParams(contentTypedName+"Request")...).Params(returnType))
			}
		}
	}

	tags := sortedMapKeys(methodsByTag)

	result := make([]jen.Code, 0, len(tags))
	for _, tag := range tags {
		result = append(result, jen.Type().Id(strings.Title(tag)+"Service").Interface(methodsByTag[tag]...))
	}
	return jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(result...)...)
}

func (generator *Generator) responseStruct() jen.Code {
	// responseInterface lets the classic-mode wrapper reach *response through
	// the per-op interface; generic mode reads &Response[B].response directly.
	return jen.Type().Id("response").Struct(
		jen.Id("statusCode").Id("int"),
		jen.Id("body").Interface(),
		jen.Id("bodyRaw").Index().Byte(),
		jen.Id("contentType").Id("string"),
		jen.Id("redirectURL").Id("string"),
		jen.Id("headers").Map(jen.Id("string")).Id("string"),
		jen.Id("cookies").Index().Qual("net/http", "Cookie"),
	).Add(jen.Line().Line()).
		Add(jen.Type().Id("responseInterface").Interface(
			jen.Id("inner").Params().Op("*").Id("response")))
}

// genericResponseTypes emits Response[B], ResponseBuilder[B], the Request
// interface, and RequestMeta — the shared per-package generic-mode types.
func (generator *Generator) genericResponseTypes() jen.Code {
	const src = `
type RequestMeta struct {
	ProcessingResult     RequestProcessingResult
	SecurityCheckResults map[SecurityScheme]string
}

func (r RequestMeta) GetProcessingResult() RequestProcessingResult {
	return r.ProcessingResult
}

func (r RequestMeta) GetSecurityCheckResults() map[SecurityScheme]string {
	return r.SecurityCheckResults
}

// Request is implemented by every x-go-generics XxxRequest. The first two
// methods come from the embedded RequestMeta; GetHeader is per-operation
// because Header is uniquely typed per endpoint.
type Request interface {
	GetProcessingResult() RequestProcessingResult
	GetSecurityCheckResults() map[SecurityScheme]string
	GetHeader() any
}

type Response[B any] struct {
	response
}

type ResponseBuilder[B any] struct {
	response
}

// NewResponse returns a builder with content-type defaulting to
// "application/json"; override via ContentType.
func NewResponse[B any]() *ResponseBuilder[B] {
	return &ResponseBuilder[B]{response: response{contentType: "application/json"}}
}

func (b *ResponseBuilder[B]) Status(code int) *ResponseBuilder[B] {
	b.response.statusCode = code
	return b
}

func (b *ResponseBuilder[B]) ContentType(ct string) *ResponseBuilder[B] {
	b.response.contentType = ct
	return b
}

// Typed content-type shortcuts for fluent chains.
func (b *ResponseBuilder[B]) ApplicationJson() *ResponseBuilder[B] {
	b.response.contentType = "application/json"
	return b
}

func (b *ResponseBuilder[B]) ApplicationXml() *ResponseBuilder[B] {
	b.response.contentType = "application/xml"
	return b
}

func (b *ResponseBuilder[B]) ApplicationOctetStream() *ResponseBuilder[B] {
	b.response.contentType = "application/octet-stream"
	return b
}

func (b *ResponseBuilder[B]) ApplicationFormUrlencoded() *ResponseBuilder[B] {
	b.response.contentType = "application/x-www-form-urlencoded"
	return b
}

func (b *ResponseBuilder[B]) TextPlain() *ResponseBuilder[B] {
	b.response.contentType = "text/plain"
	return b
}

func (b *ResponseBuilder[B]) TextHtml() *ResponseBuilder[B] {
	b.response.contentType = "text/html"
	return b
}

func (b *ResponseBuilder[B]) TextCsv() *ResponseBuilder[B] {
	b.response.contentType = "text/csv"
	return b
}

func (b *ResponseBuilder[B]) Body(body B) *ResponseBuilder[B] {
	b.response.body = body
	return b
}

func (b *ResponseBuilder[B]) BodyAny(body any) *ResponseBuilder[B] {
	b.response.body = body
	return b
}

func (b *ResponseBuilder[B]) BodyRaw(raw []byte) *ResponseBuilder[B] {
	b.response.bodyRaw = raw
	return b
}

func (b *ResponseBuilder[B]) Header(key, value string) *ResponseBuilder[B] {
	if b.response.headers == nil {
		b.response.headers = map[string]string{}
	}
	b.response.headers[key] = value
	return b
}

func (b *ResponseBuilder[B]) Cookie(c http.Cookie) *ResponseBuilder[B] {
	b.response.cookies = append(b.response.cookies, c)
	return b
}

// Redirect sets the Location target. It only takes effect when the response
// status code is 301, 302, 303, 307 or 308; respond() silently ignores the
// redirect URL for any other status, so pair it with a matching Status call.
func (b *ResponseBuilder[B]) Redirect(url string) *ResponseBuilder[B] {
	b.response.redirectURL = url
	return b
}

func (b *ResponseBuilder[B]) Build() *Response[B] {
	return &Response[B]{response: b.response}
}
`
	return jen.Op(src)
}

func (generator *Generator) responseType(name string) jen.Code {
	decapicalizedName := generator.normalizer.decapitalize(name)
	capitalizedName := strings.Title(name)

	interfaceDeclaration := jen.Type().Id(capitalizedName+"Response").Interface(
		jen.Id("responseInterface"),
		jen.Id(decapicalizedName+"Response").Params(),
	)

	declaration := jen.Type().Id(decapicalizedName + "Response").Struct(jen.Id("response"))
	// Pointer receivers on both methods so Build returns *<type> and the
	// methods are present in the value's method set.
	interfaceImplementation := jen.Func().Params(jen.Op("*").Id(decapicalizedName+"Response")).Id(decapicalizedName+"Response").Params().Block().
		Add(jen.Line(), jen.Line()).
		Add(jen.Func().Params(
			jen.Id("r").Op("*").Id(decapicalizedName + "Response")).Id("inner").Params().Op("*").Id("response").Block(
			jen.Return().Op("&").Id("r").Dot("response"),
		))

	return jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(interfaceDeclaration, declaration, interfaceImplementation)...)
}

func (generator *Generator) responseImplementationFunc(name string) jen.Code {
	return jen.Func().Params(jen.Id(strings.Title(name) + "Response")).Id(generator.normalizer.decapitalize(name) + "Response").Params().Block()
}

//if hasHeaders && hasContentTypes
//N statusCode -> headersStruct -> M contentType -> body -> assemble

//if hasHeaders && !hasContentTypes
//N statusCode -> headersStruct -> assemble

//if !hasHeaders && hasContentTypes
//N statusCode -> M contentType -> body -> assemble

// if !hasHeaders && !hasContentTypes
// N statusCode -> assemble
func (generator *Generator) responseBuilders(operationStruct operationStruct) jen.Code {
	builderConstructorName := generator.builderConstructorName(operationStruct.Name)
	statusCodesBuilderName := generator.statusCodesBuilderName(operationStruct.PrivateName)

	structBuilder := jen.Type().Id(statusCodesBuilderName).Struct(jen.Id("response"))
	structConstructor := jen.Func().Id(builderConstructorName).Params().Params(
		jen.Op("*").Id(statusCodesBuilderName)).Block(
		jen.Return().Id("new").Call(jen.Id(statusCodesBuilderName)),
	)

	var results []jen.Code

	// Sort responses by status code to ensure deterministic response builder generation order
	var sortedResponses []operationResponse
	for _, resp := range operationStruct.Responses {
		sortedResponses = append(sortedResponses, resp)
	}
	slices.SortFunc(sortedResponses, func(a, b operationResponse) int {
		return strings.Compare(a.StatusCode, b.StatusCode)
	})

	for _, resp := range sortedResponses {
		var responseResults []jen.Code
		hasHeaders := len(resp.Headers) > 0
		hasContentTypes := len(resp.ContentTypeBodyNameMap) > 0

		//prepend generated code in following order: assemble -> (optional: content type) -> (optional: headers) -> status codes
		var nextBuilderName string

		if hasContentTypes {
			contentTypeBuilderName := generator.contentTypeBuilderName(operationStruct.PrivateName + resp.StatusCode)
			//content-type struct
			responseResults = append(responseResults, jen.Type().Id(contentTypeBuilderName).Struct(jen.Id("response")))

			var contentTypeBodyBuild []jen.Code

			//content-types -> body -> build
			// Sort content types to ensure deterministic content type builder generation order
			var contentTypes []string
			for contentType := range resp.ContentTypeBodyNameMap {
				contentTypes = append(contentTypes, contentType)
			}
			slices.Sort(contentTypes)

			for _, contentTypeName := range contentTypes {
				contentType := resp.ContentTypeBodyNameMap[contentTypeName]
				var result []jen.Code

				bodyBuilderName := generator.bodyGeneratorName(operationStruct.PrivateName+resp.StatusCode, contentTypeName)
				assemblerName := generator.assemblerName(operationStruct.Name + resp.StatusCode + generator.normalizer.contentType(contentTypeName))

				result = append(result, generator.responseContentTypeBuilder(contentTypeName, contentType, contentTypeBuilderName, bodyBuilderName, assemblerName, resp.Headers)...)

				//assembler struct, build
				responseResults = append(responseResults, generator.responseAssembler(assemblerName, operationStruct.InterfaceResponseName, operationStruct.ResponseName)...)

				contentTypeBodyBuild = append(contentTypeBodyBuild, jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(result...)...))
			}

			responseResults = generator.normalizer.doubleLineAfterEachElement(append(responseResults, contentTypeBodyBuild...)...)
			nextBuilderName = contentTypeBuilderName
		} else {
			//assembler struct, build
			assemblerName := generator.assemblerName(operationStruct.Name + resp.StatusCode)
			responseResults = append(responseResults, generator.responseAssembler(assemblerName, operationStruct.InterfaceResponseName, operationStruct.ResponseName)...)
			nextBuilderName = assemblerName
		}

		if resp.SetCookie {
			cookiesBuilderName := generator.cookiesBuilderName(operationStruct.PrivateName + resp.StatusCode)
			responseResults = append(generator.responseCookiesBuilder(cookiesBuilderName, nextBuilderName), responseResults...)
			nextBuilderName = cookiesBuilderName
		}

		if hasHeaders {
			headersStructName := generator.headersStructName(operationStruct.Name + resp.StatusCode)
			headersBuilderName := generator.headersBuilderName(operationStruct.PrivateName + resp.StatusCode)
			responseResults = append(generator.responseHeadersBuilder(resp.Headers, headersStructName, headersBuilderName, nextBuilderName), responseResults...)
			nextBuilderName = headersBuilderName
		}

		responseResults = append(generator.responseStatusCodeBuilder(resp, statusCodesBuilderName, nextBuilderName), responseResults...)
		results = append(results, responseResults...)
	}

	return jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(append([]jen.Code{structBuilder, structConstructor}, results...)...)...)
}

func (generator *Generator) responseContentTypeBuilder(contentTypeName string, contentType string, contentTypeBuilderName string, bodyBuilderName string, nextBuilderName string, headers map[string]*openapi3.HeaderRef) (results []jen.Code) {
	contentTypeFuncName := generator.contentTypeFuncName(contentTypeName)
	results = append(results, jen.Func().Params(
		jen.Id("builder").Op("*").Id(contentTypeBuilderName)).Id(contentTypeFuncName).Params().Params(
		jen.Op("*").Id(bodyBuilderName)).Block(
		jen.Id("builder").Dot("response").Dot("contentType").Op("=").Lit(contentTypeName),
		jen.Line().Return().Op("&").Id(bodyBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
	))

	results = append(results, jen.Type().Id(bodyBuilderName).Struct(
		jen.Id("response"),
	))

	results = append(results, jen.Func().Params(
		jen.Id("builder").Op("*").Id(bodyBuilderName)).Id("BodyBytesWithEncoding").Params(
		jen.Id("encoding").String(), jen.Id("body").Index().Byte()).Params(
		jen.Op("*").Id(nextBuilderName)).Block(
		jen.Id("builder").Dot("response").Dot("bodyRaw").Op("=").Id("body"),
		jen.If(jen.Id("builder").Dot("response").Dot("headers").Op("==").Nil()).Block(
			jen.Id("builder").Dot("response").Dot("headers").Op("=").Make(jen.Map(jen.String()).String()),
		),
		jen.Id("builder").Dot("response").Dot("headers").Index(jen.Lit("Content-Encoding")).Op("=").Id("encoding"),
		jen.Line().Return().Op("&").Id(nextBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response"))))

	results = append(results, jen.Func().Params(
		jen.Id("builder").Op("*").Id(bodyBuilderName)).Id("BodyBytes").Params(
		jen.Id("body").Index().Byte()).Params(
		jen.Op("*").Id(nextBuilderName)).Block(
		jen.Id("builder").Dot("response").Dot("bodyRaw").Op("=").Id("body"),
		jen.Line().Return().Op("&").Id(nextBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
	))

	results = append(results, jen.Func().Params(
		jen.Id("builder").Op("*").Id(bodyBuilderName)).Id("Body").Params(
		jen.Id("body").Qual(generator.config.ComponentsPackage, contentType)).Params(
		jen.Op("*").Id(nextBuilderName)).Block(
		jen.Id("builder").Dot("response").Dot("body").Op("=").Id("body"),
		jen.Line().Return().Op("&").Id(nextBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
	))

	return results
}

func (generator *Generator) responseStatusCodeBuilder(resp operationResponse, builderName string, nextBuilderName string) (results []jen.Code) {
	hasContentTypes := len(resp.ContentTypeBodyNameMap) > 0
	isRedirect := slices.Contains([]string{"301", "302", "303", "307", "308"}, resp.StatusCode) && !hasContentTypes

	if isRedirect {
		results = append(results, jen.Func().Params(
			jen.Id("builder").Op("*").Id(builderName)).Id("StatusCode"+resp.StatusCode).Params(jen.Id("redirectURL").String()).Params(
			jen.Op("*").Id(nextBuilderName)).Block(
			jen.Id("builder").Dot("response").Dot("statusCode").Op("=").Lit(mustParseStatusCode(resp.StatusCode)),
			jen.Id("builder").Dot("response").Dot("redirectURL").Op("=").Id("redirectURL"),
			jen.Line().Return().Op("&").Id(nextBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
		))
	} else {
		results = append(results, jen.Func().Params(
			jen.Id("builder").Op("*").Id(builderName)).Id("StatusCode"+resp.StatusCode).Params().Params(
			jen.Op("*").Id(nextBuilderName)).Block(
			jen.Id("builder").Dot("response").Dot("statusCode").Op("=").Lit(mustParseStatusCode(resp.StatusCode)),
			jen.Line().Return().Op("&").Id(nextBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
		))
	}
	return
}

func (generator *Generator) responseHeadersBuilder(headers map[string]*openapi3.HeaderRef, headersStructName string, headersBuilderName string, nextBuilderName string) (results []jen.Code) {
	//headers struct
	results = append(results, generator.headersStruct(headersStructName, headers))

	//headers builder struct
	results = append(results, jen.Type().Id(headersBuilderName).Struct(jen.Id("response")))

	//headers builder.Headers(...)
	results = append(results,
		jen.Func().Params(
			jen.Id("builder").Op("*").Id(headersBuilderName)).Id("Headers").Params(
			jen.Id("headers").Id(headersStructName)).Params(
			jen.Op("*").Id(nextBuilderName)).Block(
			jen.Id("builder").Dot("headers").Op("=").Id("headers").Dot("toMap").Call(),
			jen.Line().Return().Op("&").Id(nextBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response")),
		))
	return
}

func (generator *Generator) responseCookiesBuilder(cookieBuilderName string, nextBuilderName string) (results []jen.Code) {
	//headers builder struct
	results = append(results, jen.Type().Id(cookieBuilderName).Struct(jen.Id("response")))

	//headers builder.SetCookie(...)
	results = append(results,
		jen.Func().Params(jen.Id("builder").Op("*").Id(cookieBuilderName)).
			Id("SetCookie").Params(
			jen.Id("cookie").Op("...").Qual("net/http", "Cookie")).
			Params(jen.Op("*").Id(nextBuilderName)).Block(
			jen.Id("builder").Dot("cookies").Op("=").Id("cookie"),
			jen.Return().Op("&").Id(nextBuilderName).Values(jen.Id("response").Op(":").Id("builder").Dot("response"))))
	return
}

func (generator *Generator) responseAssembler(assemblerName string, interfaceResponseName string, responseName string) (results []jen.Code) {
	results = append(results, jen.Type().Id(assemblerName).Struct(jen.Id("response")))

	// Build returns *responseName so its pointer-receiver methods are part
	// of the value's method set.
	results = append(results, jen.Func().Params(
		jen.Id("builder").Op("*").Id(assemblerName)).Id("Build").Params().Params(
		jen.Id(interfaceResponseName)).Block(
		jen.Return().Op("&").Id(responseName).Values(jen.Id("response").Op(":").Id("builder").Dot("response"))),
	)
	return
}

func (generator *Generator) securitySchemas(swagger *openapi3.T) jen.Code {
	code := jen.Type().Id("SecurityScheme").Id("string").Line().Line()

	entries := sortedMapEntries(swagger.Components.SecuritySchemes)

	consts := make([]jen.Code, 0, len(entries))
	for _, entry := range entries {
		name := strings.Title(entry.Key)
		consts = append(consts, jen.Id("SecurityScheme"+name).Id("SecurityScheme").Op("=").Lit(name))
	}

	code = code.Const().Defs(consts...).Line().Line()

	code = code.Line().Line().
		Type().Id("securityProcessor").Struct(
		jen.Id("scheme").Id("SecurityScheme"),
		jen.Id("extract").Func().Params(jen.Id("r").Op("*").Qual("net/http", "Request")).
			Params(jen.Id("string"), jen.Id("string"), jen.Id("bool")),
		jen.Id("handle").Func().Params(jen.Id("r").Op("*").Qual("net/http", "Request"),
			jen.Id("scheme").Id("SecurityScheme"), jen.Id("name").Id("string"),
			jen.Id("value").Id("string")).Params(
			jen.Id("error")))

	extractorsHeadersFuncs := make([]jen.Code, 0, len(entries)+1)
	for _, entry := range entries {
		name := generator.normalizer.normalize(entry.Key)
		schema := entry.Value

		if schema.Value.Type == "http" {
			ifStatement := jen.Null()
			assignment := jen.Null()
			if schema.Value.Scheme == "bearer" {
				ifStatement = ifStatement.Op("!").Qual("strings", "HasPrefix").Call(jen.Id("value"), jen.Lit("Bearer "))
				assignment = assignment.Id("value").Op("=").Id("value").Index(jen.Lit(7), jen.Empty())
			} else {
				ifStatement = ifStatement.Op("!").Qual("strings", "HasPrefix").Call(jen.Id("value"), jen.Lit("Basic "))
				assignment = assignment.Id("value").Op("=").Id("value").Index(jen.Lit(6), jen.Empty())
			}

			extractorsHeadersFuncs = append(extractorsHeadersFuncs,
				jen.Line().Id("SecurityScheme"+strings.Title(name)).Op(":").Func().Params(
					jen.Id("r").Op("*").Qual("net/http", "Request")).Params(jen.Id("string"), jen.Id("string"),
					jen.Id("bool")).Block(
					jen.Id("value").Op(":=").Id("r").Dot("Header").Dot("Get").Call(jen.Lit("Authorization")).Line(),
					jen.If(ifStatement).Block(jen.Return().List(jen.Lit(""), jen.Lit(""), jen.Id("false"))).Line(),
					assignment.Line(),
					jen.Return().List(jen.Lit(schema.Value.Name), jen.Id("value"), jen.Id("value").Op("!=").Lit(""))))
			continue
		}

		if schema.Value.Type == "apiKey" {
			switch schema.Value.In {
			case "header":
				extractorsHeadersFuncs = append(extractorsHeadersFuncs,
					jen.Line().Id("SecurityScheme"+strings.Title(name)).Op(":").Func().Params(
						jen.Id("r").Op("*").Qual("net/http",
							"Request")).Params(
						jen.Id("string"), jen.Id("string"),
						jen.Id("bool")).Block(
						jen.Id("value").Op(":=").Id("r").Dot("Header").Dot("Get").Call(jen.Lit(schema.Value.Name)).Line(),
						jen.Return().List(jen.Lit(schema.Value.Name), jen.Id("value"),
							jen.Id("value").Op("!=").Lit(""))))
			case "cookie":
				extractorsHeadersFuncs = append(extractorsHeadersFuncs,
					jen.Line().Id("SecurityScheme"+strings.Title(name)).Op(":").Func().Params(
						jen.Id("r").Op("*").Qual("net/http",
							"Request")).Params(
						jen.Id("string"), jen.Id("string"),
						jen.Id("bool")).Block(
						jen.List(jen.Id("cookie"), jen.Id("err")).
							Op(":=").Id("r").Dot("Cookie").Call(jen.Lit(schema.Value.Name)).Line(),
						jen.If(jen.Id("err").Op("!=").Id("nil")).Block(jen.Return().List(jen.Lit(""), jen.Lit(""), jen.Id("false"))).Line(),
						jen.Return().List(jen.Id("cookie").Dot("Name"), jen.Id("cookie").Dot("Value"), jen.Id("true"))))
			}
			continue
		}

		extractorsHeadersFuncs = append(extractorsHeadersFuncs, jen.Null())
	}

	extractorsHeadersFuncs = append(extractorsHeadersFuncs, jen.Line())

	code = code.Line().Line().Var().Id("securityExtractorsFuncs").Op("=").Map(jen.Id("SecurityScheme")).Func().Params(
		jen.Id("r").Op("*").Qual("net/http", "Request")).Params(jen.Id("string"), jen.Id("string"),
		jen.Id("bool")).Values(extractorsHeadersFuncs...)

	interfaceFuncs := make([]jen.Code, 0, len(entries))
	for _, entry := range entries {
		name := entry.Key
		interfaceFuncs = append(interfaceFuncs,
			jen.Id("SecurityScheme"+strings.Title(name)).Params(
				jen.Id("r").Op("*").Qual("net/http",
					"Request"),
				jen.Id("scheme").Id("SecurityScheme"),
				jen.Id("name").Id("string"),
				jen.Id("value").Id("string")).Params(
				jen.Id("error")))
	}

	code = code.Line().Line().Type().Id("SecuritySchemas").Interface(interfaceFuncs...)

	code = code.Line().Line().Type().Id("SecurityCheckResult").Struct(
		jen.Id("Scheme").Id("SecurityScheme"),
		jen.Id("Value").Id("string"),
	)

	return code
}

func (generator *Generator) headersStruct(name string, headers map[string]*openapi3.HeaderRef) jen.Code {
	if len(headers) == 0 {
		return jen.Null()
	}

	entries := sortedMapEntries(headers)

	headersCode := make([]jen.Code, 0, len(entries))
	headersMapCode := make([]jen.Code, 0, len(entries))
	for _, entry := range entries {
		key := entry.Key
		fieldName := generator.normalizer.normalize(key)

		field := jen.Id(fieldName)
		generator.typee.fillGoType(field, "", fieldName, entry.Value.Value.Schema, false, false)
		headersCode = append(headersCode, field)

		headersMapCode = append(headersMapCode,
			jen.Lit(key).Op(":").Qual("github.com/spf13/cast", "ToString").Call(jen.Id("headers").Dot(fieldName)))
	}

	headersStruct := jen.Type().Id(name).Struct(headersCode...)
	headersToMap := jen.Func().Params(
		jen.Id("headers").Id(name)).Id("toMap").Params().Params(
		jen.Map(jen.Id("string")).Id("string")).Block(
		jen.Return().Map(jen.Id("string")).Id("string").
			Values(headersMapCode...))

	return jen.Null().Add(generator.normalizer.doubleLineAfterEachElement(headersStruct, headersToMap)...)
}

func (generator *Generator) specCode(swagger *openapi3.T) jen.Code {
	specJson, err := json.Marshal(swagger)
	if err != nil {
		panic(err)
	}

	minifiedJson, err := minify.JSON(string(specJson))
	if err != nil {
		panic(err)
	}

	return jen.Var().Id("spec").Op("=").Index().Id("byte").Call(jen.Lit(minifiedJson)).
		Line().Line().
		Func().Id("Spec").Params(
		jen.Id("w").Qual("net/http",
			"ResponseWriter"),
		jen.Id("_").Op("*").Qual("net/http",
			"Request")).Block(
		jen.Id("w").Dot("Header").Call().Dot("Add").Call(jen.Lit("Content-Type"),
			jen.Lit("application/json")),
		jen.Id("w").Dot("Write").Call(jen.Id("spec")))
}

func (*Generator) builderConstructorName(name string) string {
	return name + "ResponseBuilder"
}

func (*Generator) statusCodesBuilderName(name string) string {
	return name + "StatusCodeResponseBuilder"
}

func (*Generator) headersBuilderName(name string) string {
	return name + "HeadersBuilder"
}

func (*Generator) headersStructName(name string) string {
	return name + "Headers"
}

func (*Generator) cookiesBuilderName(name string) string {
	return name + "CookiesBuilder"
}

func (*Generator) assemblerName(name string) string {
	return name + "ResponseBuilder"
}

func (generator *Generator) contentTypeBuilderName(name string) string {
	return name + "ContentTypeBuilder"
}

func (generator *Generator) contentTypeFuncName(contentType string) string {
	return generator.normalizer.contentType(contentType)
}

func (generator *Generator) bodyGeneratorName(name string, contentType string) string {
	return name + generator.normalizer.contentType(contentType) + "BodyBuilder"
}

// shouldUseExternalTypeAlias determines if an enum schema should use a type alias to an external type
func (generator *Generator) shouldUseExternalTypeAlias(schema *openapi3.Schema) bool {
	// Check if schema has x-go-type extension (explicit external type mapping)
	if generator.typee.hasXGoType(schema) {
		return true
	}

	// Check for well-known formats that map to external types
	if isSchemaType(schema.Type, "string") && schema.Format != "" {
		switch schema.Format {
		case "iso3166-alpha-2", "iso3166-alpha-3", "iso4217-currency-code":
			return true
		}
	}

	return false
}

// generateExternalTypeAlias generates a type alias to an external type
func (generator *Generator) generateExternalTypeAlias(name string, schema *openapi3.SchemaRef) jen.Code {
	if schema == nil || schema.Value == nil {
		return jen.Null()
	}

	v := schema.Value
	typeAlias := jen.Type().Id(name).Op("=")

	// Check if there's an explicit x-go-type mapping
	if pkg, typeName, ok := generator.typee.getXGoType(v); ok {
		if pkg == "" {
			typeAlias.Id(typeName)
		} else {
			typeAlias.Qual(pkg, typeName)
		}
		return typeAlias
	}

	// Handle well-known formats
	if isSchemaType(v.Type, "string") && v.Format != "" {
		switch v.Format {
		case "iso3166-alpha-2":
			typeAlias.Qual("github.com/mikekonan/go-types/v2/country", "Alpha2Code")
		case "iso3166-alpha-3":
			typeAlias.Qual("github.com/mikekonan/go-types/v2/country", "Alpha3Code")
		case "iso4217-currency-code":
			typeAlias.Qual("github.com/mikekonan/go-types/v2/currency", "Code")
		default:
			// Fallback to string if format is unknown
			typeAlias.String()
		}
		return typeAlias
	}

	// Fallback - shouldn't reach here if shouldUseExternalTypeAlias returned true
	typeAlias.String()
	return typeAlias
}

func (generator *Generator) trimPackagePath(from string) string {
	index := strings.LastIndex(from, "/")
	if index < 0 {
		return from
	}

	return from[index+1:]
}
