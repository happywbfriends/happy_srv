package v1

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type EnrichDataProvider func(name string) (value any, found bool)

/*
 Заполняет значения свойств структуры, на которую указывает obj, исходя из тегов и провайдера данных.
 obj должен быть *struct
 Проходит по всем публичным свойствам объекта (без рекурсии), ищет для каждого свойства тег с именем tagName.
 Если такой тег находится, пытается взять его значение из `prov`.
 Если получилось, то устанавливает.

	Преобразование типов.
	Предусматривается только преобразование из строки в любой тип, потому что многие провайдеры
	предоставляют только строки (Query, Headers).
	Любые другие преобразования ведут к ошибке.
*/
func Enrich(obj any, tagName string, prov EnrichDataProvider) (nbTagsFound int, e error) {
	// TODO: очень сырой код рефлексии
	v := reflect.ValueOf(obj)

	if v.Kind() != reflect.Ptr {
		panic(v)
	}

	v = v.Elem()
	if !v.IsValid() { // nil ptr
		panic(v)
	}

	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		tf := t.Field(i)
		vf := v.Field(i)

		if alias, ok := tf.Tag.Lookup(tagName); ok {
			if !vf.CanSet() {
				panic(fmt.Sprintf("Field %v is marked with alias but is not settable", tf))
			}

			nbTagsFound++

			// Пробел после запятой - норм, до запятой - не норм:
			// alias:"x,required"	ok
			// alias:"x, required"	ok
			// alias:"x ,required"	error
			required := false
			notEmpty := false
			if before, after, found := strings.Cut(alias, ","); found {
				after = strings.TrimSpace(after)
				if after == "required" {
					required = true
				} else if after == "notEmpty" {
					notEmpty = true
				} else {
					return 0, fmt.Errorf("invalid tag syntax, expected 'required': %s", alias)
				}
				alias = before
			}

			value, found := prov(alias)
			if found {
				switch vf.Kind() {
				case reflect.String:
					if strVal, ok := value.(string); ok {

						if notEmpty && strVal == "" {
							return 0, fmt.Errorf("empty value for tag '%s'", alias)
						}

						vf.SetString(strVal)
					} else {
						return 0, fmt.Errorf("value %v is not a string", value)
					}

				case reflect.Int:
					if intVal, ok := value.(int); ok {
						if notEmpty && intVal == 0 {
							return 0, fmt.Errorf("empty value (zero) for tag '%s'", alias)
						}
						vf.SetInt(int64(intVal))
					} else if strVal, ok := value.(string); ok {
						intVal, err := strconv.Atoi(strVal)
						if err != nil {
							return 0, err
						}
						if notEmpty && intVal == 0 {
							return 0, fmt.Errorf("empty value (zero) for tag '%s'", alias)
						}
						vf.SetInt(int64(intVal))
					} else {
						return 0, fmt.Errorf("value %v cannot be converted to int", value)
					}
				default:
					panic(fmt.Sprintf("Field %v is is not supported", tf))
				}
			} else if required || notEmpty {
				return 0, fmt.Errorf("required value '%s' missing", alias)
			}
		}
	}

	return
}
