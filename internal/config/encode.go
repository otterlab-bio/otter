package config

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

func MarshalWithMapstructureTags(value interface{}) ([]byte, error) {
	mappedValue, err := encodeMapstructureValue(reflect.ValueOf(value))
	if err != nil {
		return nil, err
	}
	encoded, err := yaml.Marshal(mappedValue)
	if err != nil {
		return nil, fmt.Errorf("marshal mapstructure YAML: %w", err)
	}
	return encoded, nil
}

func encodeMapstructureValue(value reflect.Value) (interface{}, error) {
	if !value.IsValid() {
		return nil, nil
	}
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		encodedStruct := make(map[string]interface{})
		valueType := value.Type()
		for fieldIndex := 0; fieldIndex < value.NumField(); fieldIndex++ {
			fieldType := valueType.Field(fieldIndex)
			if fieldType.PkgPath != "" {
				continue
			}
			fieldName, omitEmpty, skip := mapstructureField(fieldType)
			if skip {
				continue
			}
			fieldValue := value.Field(fieldIndex)
			if omitEmpty && fieldValue.IsZero() {
				continue
			}
			encodedValue, err := encodeMapstructureValue(fieldValue)
			if err != nil {
				return nil, fmt.Errorf("encode field %s: %w", fieldType.Name, err)
			}
			encodedStruct[fieldName] = encodedValue
		}
		return encodedStruct, nil
	case reflect.Slice, reflect.Array:
		encodedSlice := make([]interface{}, 0, value.Len())
		for itemIndex := 0; itemIndex < value.Len(); itemIndex++ {
			encodedItem, err := encodeMapstructureValue(value.Index(itemIndex))
			if err != nil {
				return nil, err
			}
			encodedSlice = append(encodedSlice, encodedItem)
		}
		return encodedSlice, nil
	case reflect.Map:
		encodedMap := make(map[interface{}]interface{}, value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			encodedKey, err := encodeMapstructureValue(iterator.Key())
			if err != nil {
				return nil, err
			}
			encodedValue, err := encodeMapstructureValue(iterator.Value())
			if err != nil {
				return nil, err
			}
			encodedMap[encodedKey] = encodedValue
		}
		return encodedMap, nil
	default:
		return value.Interface(), nil
	}
}

func mapstructureField(field reflect.StructField) (string, bool, bool) {
	tag := field.Tag.Get("mapstructure")
	if tag == "-" {
		return "", false, true
	}
	parts := strings.Split(tag, ",")
	fieldName := parts[0]
	if fieldName == "" {
		fieldName = strings.ToLower(field.Name)
	}
	omitEmpty := false
	for _, option := range parts[1:] {
		if option == "omitempty" {
			omitEmpty = true
		}
	}
	return fieldName, omitEmpty, false
}
