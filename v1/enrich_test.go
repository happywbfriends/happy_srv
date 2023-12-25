package v1

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type EnrichTest struct {
	Str string `alias:"Str"`
	Int int    `alias:"Int"`
}

type EnrichTestRequired1 struct {
	Str string `alias:"Str,required"`
}
type EnrichTestRequired2 struct {
	Str string `alias:"Str, required"`
}
type EnrichTestRequired3 struct {
	Str string `alias:"Str ,required"`
}

type EmptyStruct struct{}

func TestEnrich(t *testing.T) {
	var strVal = "str"
	var intVal = 123

	tests := []struct {
		obj     any            // куда парсим
		tagName string         // на какой тег ориентируемся
		vals    map[string]any // из каких значений парсим
		result  any            // что должны получить на выходе
		errText string         // если предполагается ошибка, то какой текст в ней должен быть
	}{
		{&EnrichTest{}, "alias", map[string]any{"Str": "a", "Int": 1}, &EnrichTest{Str: "a", Int: 1}, ""},
		// поиск должен быть регистрозависимым (поле Str не будет найдено)
		{&EnrichTest{}, "alias", map[string]any{"str": "a", "Int": 1}, &EnrichTest{Str: "", Int: 1}, ""},

		// преобразование ->str
		{&EnrichTest{}, "alias", map[string]any{"Str": 123}, &EnrichTest{}, "is not a"},
		{&EnrichTest{}, "alias", map[string]any{"Str": 123.45}, &EnrichTest{}, "is not a"},
		{&EnrichTest{}, "alias", map[string]any{"Str": true}, &EnrichTest{}, "is not a"},
		{&EnrichTest{}, "alias", map[string]any{"Str": nil}, &EnrichTest{}, "is not a"},
		{&EnrichTest{}, "alias", map[string]any{"Str": EmptyStruct{}}, &EnrichTest{}, "is not a"},
		{&EnrichTest{}, "alias", map[string]any{"Str": &EmptyStruct{}}, &EnrichTest{}, "is not a"},
		{&EnrichTest{}, "alias", map[string]any{"Str": &strVal}, &EnrichTest{}, "is not a"},
		{&EnrichTest{}, "alias", map[string]any{"Str": &intVal}, &EnrichTest{}, "is not a"},

		// преобразование ->int
		{&EnrichTest{}, "alias", map[string]any{"Int": "str"}, &EnrichTest{}, "invalid syntax"},
		{&EnrichTest{}, "alias", map[string]any{"Int": "123"}, &EnrichTest{Int: 123}, ""},
		{&EnrichTest{}, "alias", map[string]any{"Int": 123.45}, &EnrichTest{}, "cannot be converted to int"},
		{&EnrichTest{}, "alias", map[string]any{"Int": true}, &EnrichTest{}, "cannot be converted to int"},
		{&EnrichTest{}, "alias", map[string]any{"Int": nil}, &EnrichTest{}, "cannot be converted to int"},
		{&EnrichTest{}, "alias", map[string]any{"Int": EmptyStruct{}}, &EnrichTest{}, "cannot be converted to int"},
		{&EnrichTest{}, "alias", map[string]any{"Int": &EmptyStruct{}}, &EnrichTest{}, "cannot be converted to int"},
		{&EnrichTest{}, "alias", map[string]any{"Int": &strVal}, &EnrichTest{}, "cannot be converted to int"},
		{&EnrichTest{}, "alias", map[string]any{"Int": &intVal}, &EnrichTest{}, "cannot be converted to int"},

		// Required
		{&EnrichTestRequired1{}, "alias", map[string]any{"Str": "a"}, &EnrichTestRequired1{Str: "a"}, ""},
		{&EnrichTestRequired1{}, "alias", map[string]any{"Str": ""}, &EnrichTestRequired1{Str: ""}, ""}, // пустое значение норм
		{&EnrichTestRequired2{}, "alias", map[string]any{"Str": "a"}, &EnrichTestRequired2{Str: "a"}, ""},
		{&EnrichTestRequired3{}, "alias", map[string]any{"Str": "a"}, &EnrichTestRequired3{}, "missing"},
		{&EnrichTestRequired1{}, "alias", map[string]any{}, &EnrichTestRequired1{}, "missing"},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := Enrich(tt.obj, tt.tagName, func(name string) (value any, found bool) {
				value, found = tt.vals[name]
				return
			})

			if tt.result != nil {
				if !reflect.DeepEqual(tt.obj, tt.result) {
					t.Errorf("got = %+v, want %+v", tt.obj, tt.result)
				}
			}

			if tt.errText != "" {
				if err == nil {
					t.Errorf("error = %v, want '%s'", err, tt.errText)
				} else if !strings.Contains(err.Error(), tt.errText) {
					t.Errorf("error = %v, want '%s'", err, tt.errText)
				}
			} else if err != nil {
				t.Errorf("error = %v", err)
			}
		})
	}
}

type EnrichBenchmark struct {
	Str1 string `alias:"Str1"`
	Str2 string `alias:"Str2"`
	Str3 string `alias:"Str3"`
	Int1 int    `alias:"Int1"`
	Int2 int    `alias:"Int2"`
	Int3 int    `alias:"Int3"`
}

func BenchmarkEnrich(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var b EnrichBenchmark
		_, _ = Enrich(&b, "alias", func(name string) (value any, found bool) {
			return "12345", true
		})
	}
}
