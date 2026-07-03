package gosh

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestInvokeLegacyCallShapes(t *testing.T) {
	script := testScript(t.TempDir())

	calledNoArgs := false
	if err := invokeLegacyCall(script, Call{Name: "noArgs", Func: reflect.ValueOf(func() {
		calledNoArgs = true
	})}, ""); err != nil {
		t.Fatal(err)
	}
	if !calledNoArgs {
		t.Fatalf("no-args function was not called")
	}

	calledScript := false
	if err := invokeLegacyCall(script, Call{Name: "scriptOnly", Func: reflect.ValueOf(func(s *Script) {
		calledScript = s == script
	})}, ""); err != nil {
		t.Fatal(err)
	}
	if !calledScript {
		t.Fatalf("script-only function did not receive script")
	}

	var gotRaw string
	if err := invokeLegacyCall(script, Call{Name: "scriptRaw", Func: reflect.ValueOf(func(_ *Script, raw string) {
		gotRaw = raw
	})}, "hello world"); err != nil {
		t.Fatal(err)
	}
	if gotRaw != "hello world" {
		t.Fatalf("raw args = %q", gotRaw)
	}

	if err := invokeLegacyCall(script, Call{Name: "tooMany", Func: reflect.ValueOf(func(string, string) {})}, "x"); err == nil {
		t.Fatalf("expected too many params error")
	}
	if err := invokeLegacyCall(script, Call{Name: "tooManyScript", Func: reflect.ValueOf(func(*Script, string, string) {})}, "x"); err == nil {
		t.Fatalf("expected too many script params error")
	}
}

func TestInvokeStructuredCallConversionsAndErrors(t *testing.T) {
	type customString string
	var got struct {
		name   customString
		ok     bool
		count  int8
		amount uint16
		ratio  float32
	}
	call := Call{
		Name: "structured",
		Func: reflect.ValueOf(func(name customString, ok bool, count int8, amount uint16, ratio float32) {
			got.name = name
			got.ok = ok
			got.count = count
			got.amount = amount
			got.ratio = ratio
		}),
		Tool: ToolSpec{
			Name:       "structured",
			Structured: true,
			Params: []ParamSpec{
				{Name: "name", Type: "string", Required: true},
				{Name: "ok", Type: "boolean", Required: true},
				{Name: "count", Type: "integer", Required: true},
				{Name: "amount", Type: "integer", Required: true},
				{Name: "ratio", Type: "number", Required: true},
			},
		},
	}

	if err := invokeCall(testScript(t.TempDir()), call, "", []string{"bob", "true", "7", "9", "1.5"}); err != nil {
		t.Fatal(err)
	}
	if got.name != "bob" || !got.ok || got.count != 7 || got.amount != 9 || got.ratio != 1.5 {
		t.Fatalf("converted values = %+v", got)
	}

	if err := invokeCall(testScript(t.TempDir()), call, "", []string{"bob"}); err == nil {
		t.Fatalf("expected validation error")
	}
	if err := invokeStructuredCall(testScript(t.TempDir()), call, []string{"bob", "not-bool", "7", "9", "1.5"}); err == nil {
		t.Fatalf("expected conversion error")
	}
	badTarget := call
	badTarget.Func = reflect.ValueOf(func([]string) {})
	badTarget.Tool.Params = []ParamSpec{{Name: "items", Type: "string", Required: true}}
	if err := invokeStructuredCall(testScript(t.TempDir()), badTarget, []string{"x"}); err == nil {
		t.Fatalf("expected unsupported target error")
	}
}

func TestValidateParamValueErrors(t *testing.T) {
	cases := []ParamSpec{
		{Name: "ok", Type: "boolean"},
		{Name: "count", Type: "integer"},
		{Name: "ratio", Type: "number"},
		{Name: "custom", Type: "object"},
	}
	for _, tc := range cases {
		if err := validateParamValue(tc, "not-valid"); err == nil {
			t.Fatalf("expected error for %+v", tc)
		}
	}
	if err := validateParamValue(ParamSpec{Name: "plain", Type: "string"}, "anything"); err != nil {
		t.Fatalf("string validation failed: %v", err)
	}
}

func TestCollectCallError(t *testing.T) {
	want := errors.New("boom")
	results := reflect.ValueOf(func() (int, error) {
		return 1, want
	}).Call(nil)
	if err := collectCallError(results); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	empty := reflect.ValueOf(func() error { return nil }).Call(nil)
	if err := collectCallError(empty); err != nil {
		t.Fatalf("nil error result returned %v", err)
	}
}

func TestOptionalParamDescAndSchema(t *testing.T) {
	spec := ToolSpec{Name: "schema", Structured: true}
	Param("name", ParamDesc("person name"))(&spec)
	Param("title", Optional())(&spec)
	schema := spec.inputSchema()
	props := schema["properties"].(map[string]interface{})
	name := props["name"].(map[string]interface{})
	if name["description"] != "person name" {
		t.Fatalf("param description missing: %#v", name)
	}
	required := schema["required"].([]string)
	if strings.Join(required, ",") != "name" {
		t.Fatalf("required = %#v", required)
	}
	if jsonTypeForReflect(reflect.TypeOf(true)) != "boolean" ||
		jsonTypeForReflect(reflect.TypeOf(uint(1))) != "integer" ||
		jsonTypeForReflect(reflect.TypeOf(1.2)) != "number" {
		t.Fatalf("unexpected reflected json types")
	}
}
