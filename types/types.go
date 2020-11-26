package types

import (
	"github.com/dave/jennifer/jen"
	"github.com/getkin/kin-openapi/openapi3"
)

type Parameter struct {
	Name   string
	Type   *jen.Statement
	Origin *openapi3.ParameterRef
}

//
//type PathParam struct {
//	Parameter
//}
//
//type QueryParam struct {
//	Parameter
//}
//
//type ReqHeadParam struct {
//	Parameter
//}
//
//type RequestBody struct {
//	Name      string
//	MediaType string
//	Required  bool
//	SchemaRef string
//	Fields    []Parameter
//}
//
//type PathEntity struct {
//	Path            string
//	Method          string
//	PathParams      []PathParam
//	QueryParams     []PathParam
//	ReqHeaderParams []ReqHeadParam
//	Tags            []string
//	Summary         string
//	Description     string
//	OperationID     string
//}
