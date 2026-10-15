package jsonvalue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

// Decode checks duplicate decoded keys, Unicode, syntax and depth, retaining
// json.Number for arbitrary-precision schema/metadata values. Parameter numeric
// limits are specific to Normalize and are deliberately not applied here.
func Decode(input []byte, maxBytes int64) (any, error) {
	if maxBytes < 0 || int64(len(input)) > maxBytes {
		return nil, errors.New("JSON exceeds limit")
	}
	if !utf8.Valid(input) {
		return nil, errors.New("JSON contains invalid UTF-8")
	}
	p := parser{input: input}
	_, err := p.value(0)
	if err != nil {
		return nil, err
	}
	p.space()
	if p.pos != len(input) {
		return nil, p.fail("trailing input")
	}
	// Decode bounds wire bytes. Canonical string escaping can expand otherwise
	// valid input; only Normalize bounds that separate representation.
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value any
	if err = decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

// Unmarshal is a strict wire DTO decoder: field names must match their json tags
// exactly, in addition to strict JSON syntax. RawMessage and interface/map values
// retain domain-owned keys and original bytes through encoding/json.Unmarshal.
func Unmarshal(input []byte, target any, maxBytes int64) error {
	typ := reflect.TypeOf(target)
	if typ == nil || typ.Kind() != reflect.Pointer || reflect.ValueOf(target).IsNil() {
		return errors.New("JSON target must be a nonnil pointer")
	}
	value, err := Decode(input, maxBytes)
	if err != nil {
		return err
	}
	if err = checkFields(value, typ.Elem()); err != nil {
		return err
	}
	return json.Unmarshal(input, target)
}
func checkFields(value any, typ reflect.Type) error {
	if typ == reflect.TypeFor[json.RawMessage]() {
		return nil
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		obj, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if !f.IsExported() {
				continue
			}
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			fields[name] = f.Type
		}
		for key, child := range obj {
			field, ok := fields[key]
			if !ok {
				return fmt.Errorf("unknown JSON field %q", key)
			}
			if err := checkFields(child, field); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if items, ok := value.([]any); ok {
			for _, child := range items {
				if err := checkFields(child, typ.Elem()); err != nil {
					return err
				}
			}
		}
	case reflect.Map:
		if obj, ok := value.(map[string]any); ok {
			for _, child := range obj {
				if err := checkFields(child, typ.Elem()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
