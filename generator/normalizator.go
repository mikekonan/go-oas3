package generator

import (
	"reflect"
	"strings"
	"unicode"

	"github.com/ahmetb/go-linq"
	"github.com/dave/jennifer/jen"
	"github.com/spf13/cast"
)

type Normalizer struct{}

// titleCase replaces deprecated strings.Title with proper case conversion
// It capitalizes the first letter but preserves the camelCase structure
func (normalizer *Normalizer) titleCase(str string) string {
	if str == "" {
		return str
	}
	return strings.ToUpper(str[:1]) + str[1:]
}

func (normalizer *Normalizer) decapitalize(str string) string {
	return strings.ToLower(str[:1]) + str[1:]
}

func (normalizer *Normalizer) normalize(str string) string {
	separators := "-#@!$&=.+:;_~ (){}[]"
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
		if strings.ToLower(n[len(n)-4:]) == NormUUIDSuffix {
			n = n[:len(n)-4] + NormUUID
		}
	}

	if len(n) > 1 {
		if strings.ToLower(n[len(n)-2:]) == NormIDSuffix {
			n = n[:len(n)-2] + NormID
		}
	}

	return n
}

func (normalizer *Normalizer) normalizeOperationName(path string, method string) string {
	return normalizer.normalize(strings.ReplaceAll(strings.ToLower(method)+path, "/", "-"))
}

func (normalizer *Normalizer) doubleLineAfterEachElement(from ...jen.Code) (result []jen.Code) {
	linq.From(from).SelectManyT(func(code jen.Code) linq.Query {
		if reflect.DeepEqual(code, jen.Null()) || reflect.DeepEqual(code, jen.Line()) {
			return linq.From([]jen.Code{})
		}

		return linq.From([]jen.Code{code, jen.Line(), jen.Line()})
	}).ToSlice(&result)

	return
}

func (normalizer *Normalizer) lineAfterEachElement(from ...jen.Code) (result []jen.Code) {
	linq.From(from).SelectManyT(func(code jen.Code) linq.Query {
		if reflect.DeepEqual(code, jen.Null()) || reflect.DeepEqual(code, jen.Line()) {
			return linq.From([]jen.Code{})
		}

		return linq.From([]jen.Code{code, jen.Line()})
	}).ToSlice(&result)

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

	var split = func(r rune) bool {
		return r == '/' || r == '-'
	}

	return cast.ToString(linq.From(strings.FieldsFunc(str, split)).
		AggregateWithSeedT("", func(accumulator, str string) string { return accumulator + normalizer.titleCase(str) }))
}
