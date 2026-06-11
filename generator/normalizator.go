package generator

import (
	"reflect"
	"strings"
	"unicode"

	"github.com/dave/jennifer/jen"
)

type Normalizer struct{}

func (normalizer *Normalizer) decapitalize(str string) string {
	return strings.ToLower(str[:1]) + str[1:]
}

func (normalizer *Normalizer) normalize(str string) string {
	// '/' is a separator so "application/json" → "ApplicationJson", matching
	// what contentType() produces — diverging here breaks inline response
	// type-name reconstruction in the generator.
	separators := "-#@!$&=.+:;_~ (){}[]/"
	s := strings.Trim(str, " ")

	n := ""
	capNext := true
	for _, v := range s {
		if unicode.IsUpper(v) {
			n += string(v)
		}
		if unicode.IsDigit(v) {
			n += string(v)
		}
		if unicode.IsLower(v) {
			if capNext {
				n += strings.ToUpper(string(v))
			} else {
				n += string(v)
			}
		}

		if strings.ContainsRune(separators, v) {
			capNext = true
		} else {
			capNext = false
		}
	}

	if len(n) > 3 {
		if strings.ToLower(n[len(n)-4:]) == "uuid" {
			n = n[:len(n)-4] + "UUID"
		}
	}

	if len(n) > 1 {
		if strings.ToLower(n[len(n)-2:]) == "id" {
			n = n[:len(n)-2] + "ID"
		}
	}

	return n
}

func (normalizer *Normalizer) normalizeOperationName(path string, method string) string {
	return normalizer.normalize(strings.ReplaceAll(strings.ToLower(method)+path, "/", "-"))
}

// isEmptyCode reports whether code is the no-op jen.Null() / jen.Line()
// sentinel — used to skip them when interleaving blank lines.
func isEmptyCode(code jen.Code) bool {
	return reflect.DeepEqual(code, jen.Null()) || reflect.DeepEqual(code, jen.Line())
}

func (normalizer *Normalizer) doubleLineAfterEachElement(from ...jen.Code) []jen.Code {
	result := make([]jen.Code, 0, len(from)*3)
	for _, code := range from {
		if isEmptyCode(code) {
			continue
		}
		result = append(result, code, jen.Line(), jen.Line())
	}
	return result
}

func (normalizer *Normalizer) lineAfterEachElement(from ...jen.Code) []jen.Code {
	result := make([]jen.Code, 0, len(from)*2)
	for _, code := range from {
		if isEmptyCode(code) {
			continue
		}
		result = append(result, code, jen.Line())
	}
	return result
}

func (normalizer *Normalizer) extractNameFromRef(str string) string {
	if str == "" {
		return ""
	}

	return normalizer.normalize(str[strings.LastIndex(str, "/")+1:])
}

func (normalizer *Normalizer) contentType(str string) string {
	if str == "" {
		return ""
	}

	// Treat every non-alphanumeric rune as a word boundary so structured
	// MIME suffixes (application/problem+json, application/vnd.api+json)
	// produce valid Go identifiers instead of leaking `+`/`.` into the name.
	split := func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	}
	var sb strings.Builder
	for _, part := range strings.FieldsFunc(str, split) {
		sb.WriteString(strings.Title(part))
	}
	return sb.String()
}
