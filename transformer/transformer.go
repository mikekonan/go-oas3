package transformer

import (
	"fmt"
	"strings"

	"github.com/ahmetb/go-linq"
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cast"

	"github.com/mikekonan/go-oas3/configurator"
)

type Transformer struct {
	config             *configurator.Config `di.inject:"config"`
	normalizer         *Normalizer          `di.inject:"normalizer"`
	generator          *Generator           `di.inject:"generator"`
	interfaceGenerator *InterfaceGenerator  `di.inject:"interfaceGenerator"`
}

func (transformer *Transformer) Transform(swagger *openapi3.Swagger) {
	components := transformer.generator.components(swagger.Components.Schemas)

	notAnnotatedComponents := transformer.generator.componentsFromPaths(swagger.Paths)

	linq.From(components).
		Concat(linq.From(notAnnotatedComponents)).
		ToMapByT(&components,
			func(kv linq.KeyValue) interface{} { return kv.Key },
			func(kv linq.KeyValue) interface{} { return kv.Value },
		)

	requestParameters := transformer.generator.requestParameters(swagger.Paths)

	pathsCode := linq.From(requestParameters).AggregateWithSeedT("",
		func(accumulator string, kv linq.KeyValue) string { return accumulator + jen.Null().Add(transformer.normalizer.lineAfterEachCodeElement(kv.Value.([]jen.Code)...)...).GoString() })

	componentsCode := linq.From(components).
		AggregateWithSeedT("",
			func(accumulator string, kv linq.KeyValue) string { return accumulator + jen.Null().Add(transformer.normalizer.lineAfterEachCodeElement(kv.Value.(jen.Code))...).GoString() })

	builders := transformer.interfaceGenerator.Generate(swagger).GoString()
	fmt.Println(requestParameters, components, componentsCode, pathsCode, builders)
}

////todo: move it to output module
func (transformer *Transformer) generateFilesForComponentsMap(from map[string][]jen.Code) (files map[string]*jen.File) {
	linq.From(from).ToMapByT(
		&files,
		func(kv linq.KeyValue) string { return strings.ToLower(cast.ToString(kv.Key)) + ".go" },
		func(kv linq.KeyValue) (file *jen.File) {
			file = jen.NewFile("package")
			file.Add(kv.Value.([]jen.Code)...)

			return
		},
	)

	return
}
