package jsonvalue

import (
	"strings"
	"testing"
)

func TestNormalizeGolden(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{` { "z": [1.0, 1e0, -0], "a": {"b": "\u0061", "a": true} } `, `{"a":{"a":true,"b":"a"},"z":[1.0,1e0,-0]}`},
		{`{"n":9223372036854775807,"x":-9223372036854775808}`, `{"n":9223372036854775807,"x":-9223372036854775808}`},
		{`{"emoji":"\ud83d\ude00","s":"a/b\n"}`, "{\"emoji\":\"😀\",\"s\":\"a/b\\n\"}"},
	} {
		got, err := NormalizeObject([]byte(tc.in))
		if err != nil || string(got) != tc.want {
			t.Errorf("%s: got %s, %v; want %s", tc.in, got, err, tc.want)
		}
		again, err := NormalizeObject(got)
		if err != nil || string(again) != string(got) {
			t.Fatal("not idempotent", err)
		}
	}
	intValue, _ := NormalizeObject([]byte(`{"n":1}`))
	floatValue, _ := NormalizeObject([]byte(`{"n":1.0}`))
	if string(intValue) == string(floatValue) {
		t.Fatal("numeric types collapsed")
	}
}

func TestNormalizeRejectsAmbiguousOrUnsupportedJSON(t *testing.T) {
	for _, in := range []string{
		`{"a":1,"\u0061":2}`, `{"x":{"a":1,"a":2}}`, `{"x":"\ud800"}`, `{"x":"\udc00"}`, `{"x":"\ud800\u0041"}`,
		"{\"x\":\"\xff\"}", `{"n":9223372036854775808}`, `{"n":-9223372036854775809}`, `{"n":1e999}`, `{"n":01}`, `{"n":+1}`,
		`{"n":.1}`, `{"n":1.}`, `{"n":NaN}`, `{"n":1} true`, `[]`, `null`, `{"x":true,}`, `{"x":"\x00"}`, `{"x":"\u+123"}`,
	} {
		if got, err := NormalizeObject([]byte(in)); err == nil {
			t.Errorf("accepted %q as %s", in, got)
		}
	}
}

func TestNormalizeDepthAndSize(t *testing.T) {
	good := `{"x":` + strings.Repeat(`[`, 62) + `0` + strings.Repeat(`]`, 62) + `}`
	if _, err := NormalizeObject([]byte(good)); err != nil {
		t.Fatal(err)
	}
	bad := `{"x":` + strings.Repeat(`[`, 64) + `0` + strings.Repeat(`]`, 64) + `}`
	if _, err := NormalizeObject([]byte(bad)); err == nil {
		t.Fatal("accepted excessive depth")
	}
	if _, err := Normalize([]byte(`"` + strings.Repeat("x", MaxBytes) + `"`)); err == nil {
		t.Fatal("accepted excessive bytes")
	}
	if got, err := NormalizeObject([]byte(`{"x":"` + strings.Repeat("<", MaxBytes/3) + `"}`)); err == nil && len(got) > MaxBytes {
		t.Fatal("normalization exceeds its own accepted size")
	}
}

func TestNormalizeNoFloatRoundTrip(t *testing.T) {
	in := `{"fraction":9007199254740993.0,"int":9007199254740993,"tiny":1e-300}`
	got, err := NormalizeObject([]byte(in))
	if err != nil || string(got) != in {
		t.Fatalf("precision or lexemes changed: %s %v", got, err)
	}
}
