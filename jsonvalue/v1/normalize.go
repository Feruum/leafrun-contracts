// Package jsonvalue implements Leafrun JSON normalization version 1. Numeric
// lexemes are preserved: this is deliberately not RFC 8785 number normalization.
package jsonvalue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

const Version = "leafrun-json-v1"
const MaxBytes = 1 << 20
const MaxDepth = 64

// Normalize returns independently owned, deterministic JSON bytes. Integer
// tokens must fit int64. Decimal/exponent tokens must be finite binary64 values;
// conversion is used only for range checking, never to serialize their value.
func Normalize(input []byte) ([]byte, error) {
	if len(input) > MaxBytes {
		return nil, errors.New("JSON exceeds 1 MiB")
	}
	if !utf8.Valid(input) {
		return nil, errors.New("JSON contains invalid UTF-8")
	}
	p := parser{input: input, numberBounds: true}
	out, err := p.value(0)
	if err != nil {
		return nil, err
	}
	p.space()
	if p.pos != len(input) {
		return nil, p.fail("trailing input")
	}
	if len(out) > MaxBytes {
		return nil, errors.New("normalized JSON exceeds 1 MiB")
	}
	return out, nil
}

// NormalizeObject additionally requires an object, as required for parameters.
func NormalizeObject(input []byte) ([]byte, error) {
	out, err := Normalize(input)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 || out[0] != '{' {
		return nil, errors.New("JSON must be an object")
	}
	return out, nil
}

type parser struct {
	input        []byte
	pos          int
	numberBounds bool
}

func (p *parser) fail(msg string) error { return fmt.Errorf("JSON at byte %d: %s", p.pos, msg) }
func (p *parser) space() {
	for p.pos < len(p.input) {
		switch p.input[p.pos] {
		case ' ', '\t', '\r', '\n':
			p.pos++
		default:
			return
		}
	}
}
func (p *parser) take(c byte) bool {
	p.space()
	if p.pos < len(p.input) && p.input[p.pos] == c {
		p.pos++
		return true
	}
	return false
}
func (p *parser) value(depth int) ([]byte, error) {
	p.space()
	if p.pos == len(p.input) {
		return nil, p.fail("expected value")
	}
	switch p.input[p.pos] {
	case '{':
		if depth >= MaxDepth {
			return nil, p.fail("maximum depth exceeded")
		}
		p.pos++
		values := map[string][]byte{}
		if !p.take('}') {
			for {
				key, err := p.stringValue()
				if err != nil {
					return nil, err
				}
				if _, exists := values[key]; exists {
					return nil, p.fail("duplicate object key")
				}
				if !p.take(':') {
					return nil, p.fail("expected colon")
				}
				value, err := p.value(depth + 1)
				if err != nil {
					return nil, err
				}
				values[key] = value
				if p.take('}') {
					break
				}
				if !p.take(',') {
					return nil, p.fail("expected comma")
				}
			}
		}
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := []byte{'{'}
		for i, key := range keys {
			if i > 0 {
				out = append(out, ',')
			}
			encoded, _ := json.Marshal(key)
			out = append(out, encoded...)
			out = append(out, ':')
			out = append(out, values[key]...)
		}
		return append(out, '}'), nil
	case '[':
		if depth >= MaxDepth {
			return nil, p.fail("maximum depth exceeded")
		}
		p.pos++
		out := []byte{'['}
		if p.take(']') {
			return append(out, ']'), nil
		}
		for i := 0; ; i++ {
			if i > 0 {
				out = append(out, ',')
			}
			v, err := p.value(depth + 1)
			if err != nil {
				return nil, err
			}
			out = append(out, v...)
			if p.take(']') {
				break
			}
			if !p.take(',') {
				return nil, p.fail("expected comma")
			}
		}
		return append(out, ']'), nil
	case '"':
		s, err := p.stringValue()
		if err != nil {
			return nil, err
		}
		return json.Marshal(s)
	case 't', 'f', 'n':
		for _, s := range []string{"true", "false", "null"} {
			if bytes.HasPrefix(p.input[p.pos:], []byte(s)) {
				p.pos += len(s)
				return []byte(s), nil
			}
		}
		return nil, p.fail("invalid literal")
	default:
		return p.number()
	}
}

func (p *parser) stringValue() (string, error) {
	p.space()
	if p.pos == len(p.input) || p.input[p.pos] != '"' {
		return "", p.fail("expected string")
	}
	p.pos++
	var out []byte
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		p.pos++
		if c == '"' {
			return string(out), nil
		}
		if c < 0x20 {
			return "", p.fail("unescaped control character")
		}
		if c != '\\' {
			out = append(out, c)
			continue
		}
		if p.pos == len(p.input) {
			return "", p.fail("incomplete escape")
		}
		c = p.input[p.pos]
		p.pos++
		switch c {
		case '"', '\\', '/':
			out = append(out, c)
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'u':
			r, err := p.hexRune()
			if err != nil {
				return "", err
			}
			if r >= 0xd800 && r <= 0xdbff {
				if p.pos+2 > len(p.input) || p.input[p.pos] != '\\' || p.input[p.pos+1] != 'u' {
					return "", p.fail("unpaired high surrogate")
				}
				p.pos += 2
				low, err := p.hexRune()
				if err != nil {
					return "", err
				}
				if low < 0xdc00 || low > 0xdfff {
					return "", p.fail("invalid low surrogate")
				}
				r = utf16.DecodeRune(r, low)
			} else if r >= 0xdc00 && r <= 0xdfff {
				return "", p.fail("unpaired low surrogate")
			}
			out = utf8.AppendRune(out, r)
		default:
			return "", p.fail("invalid escape")
		}
	}
	return "", p.fail("unterminated string")
}
func (p *parser) hexRune() (rune, error) {
	if len(p.input)-p.pos < 4 {
		return 0, p.fail("incomplete Unicode escape")
	}
	n, err := strconv.ParseUint(string(p.input[p.pos:p.pos+4]), 16, 16)
	if err != nil {
		return 0, p.fail("invalid Unicode escape")
	}
	p.pos += 4
	return rune(n), nil
}
func (p *parser) number() ([]byte, error) {
	start := p.pos
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ',' || c == ']' || c == '}' {
			break
		}
		p.pos++
	}
	token := p.input[start:p.pos]
	if len(token) == 0 || !json.Valid(token) || token[0] != '-' && (token[0] < '0' || token[0] > '9') {
		return nil, p.fail("invalid number")
	}
	if !p.numberBounds {
		return bytes.Clone(token), nil
	}
	if bytes.ContainsAny(token, ".eE") {
		n, err := strconv.ParseFloat(string(token), 64)
		if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
			return nil, p.fail("number outside finite float range")
		}
	} else if _, err := strconv.ParseInt(string(token), 10, 64); err != nil {
		return nil, p.fail("integer outside signed 64-bit range")
	}
	return bytes.Clone(token), nil
}
