package gox

import (
	"strconv"
	"testing"
	"time"
)

var benchmarkActionSpec = ActionSpec{
	Name: "MyAction",
	Params: []ActionParamSpec{
		{Name: "name", Type: ActionString},
		{Name: "age", Type: ActionNumber},
		{Name: "createdAt", Type: ActionDate},
	},
}

var benchmarkFileSpec = ActionSpec{
	Name:   "UploadPhoto",
	Params: []ActionParamSpec{{Name: "photo", Type: ActionFile}},
}

var benchmarkJSONSpec = ActionSpec{
	Name:   "SaveUser",
	Params: []ActionParamSpec{{Name: "user", Type: ActionJSON, JSName: "User"}},
}

func BenchmarkValidateActionParamsURLencoded(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{
		"0": {"Juan"},
		"1": {"42"},
		"2": {"2026-07-05T12:34:56.123456789Z"},
	}, nil)
	var name string
	var age int
	var createdAt time.Time

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &benchmarkActionSpec, &name, &age, &createdAt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsMultipartFields(b *testing.B) {
	ctx := newActionContext("multipart/form-data; boundary=test", map[string][]string{
		"0": {"Juan"},
		"1": {"42"},
		"2": {"2026-07-05T12:34:56.123456789Z"},
	}, nil)
	var name string
	var age int
	var createdAt time.Time

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &benchmarkActionSpec, &name, &age, &createdAt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsFile(b *testing.B) {
	file := mustTestFile(b, "photo.jpg", "image/jpeg", "image-bytes")
	ctx := newActionContext("multipart/form-data; boundary=test", nil, map[string][]File{"0": {file}})
	var got File

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &benchmarkFileSpec, &got); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsFileToBytes(b *testing.B) {
	file := mustTestFile(b, "photo.txt", "text/plain", "hello")
	ctx := newActionContext("multipart/form-data; boundary=test", nil, map[string][]File{"0": {file}})
	var data []byte

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &benchmarkFileSpec, &data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsJSON(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {`{"name":"Juan","age":42}`}}, nil)
	var user actionUserPayload

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user = actionUserPayload{}
		if err := ValidateActionParams(ctx, &benchmarkJSONSpec, &user); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsNumberAlias(b *testing.B) {
	spec := ActionSpec{Name: "MyAction", Params: []ActionParamSpec{{Name: "age", Type: ActionNumber}}}
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"42"}}, nil)
	var age actionAliasAge

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &spec, &age); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsInvalidNumber(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"Juan"}, "1": {"abc"}}, nil)
	var name string
	var age int

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateActionParams(ctx, &testActionSpec, &name, &age)
	}
}

func BenchmarkActionWrapperInlineOldStyle(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{
		"0": {"Juan"},
		"1": {"42"},
		"2": {"2026-07-05T12:34:56.123456789Z"},
	}, nil)
	var name string
	var age int
	var createdAt time.Time

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var ok bool

		name, ok = ctx.Request.Form.Fields.Get("0")
		if !ok {
			b.Fatal("missing name")
		}

		if !ctx.Request.Form.Fields.Has("1") {
			b.Fatal("missing age")
		}
		ageText, ok := ctx.Request.Form.Fields.Get("1")
		if !ok {
			b.Fatal("missing age")
		}

		ageParsed, err := strconv.ParseInt(ageText, 10, 0)
		if err != nil {
			b.Fatal(err)
		}
		age = int(ageParsed)

		if !ctx.Request.Form.Fields.Has("2") {
			b.Fatal("missing createdAt")
		}

		createdAtText, ok := ctx.Request.Form.Fields.Get("2")
		if !ok {
			b.Fatal("missing createdAt")
		}

		createdAt, err = time.Parse(time.RFC3339Nano, createdAtText)
		if err != nil {
			b.Fatal(err)
		}
	}
	_, _, _ = name, age, createdAt
}

var benchmarkStringOnlySpec = ActionSpec{
	Name:   "MyAction",
	Params: []ActionParamSpec{{Name: "name", Type: ActionString}},
}

var benchmarkNumberOnlySpec = ActionSpec{
	Name:   "MyAction",
	Params: []ActionParamSpec{{Name: "age", Type: ActionNumber}},
}

var benchmarkBooleanOnlySpec = ActionSpec{
	Name:   "MyAction",
	Params: []ActionParamSpec{{Name: "enabled", Type: ActionBoolean}},
}

var benchmarkDateOnlySpec = ActionSpec{
	Name:   "MyAction",
	Params: []ActionParamSpec{{Name: "createdAt", Type: ActionDate}},
}

var benchmarkJSONArraySpec = ActionSpec{
	Name:   "SaveUsers",
	Params: []ActionParamSpec{{Name: "users", Type: ActionJSON, JSName: "User[]"}},
}

var benchmarkTwoNumbersSpec = ActionSpec{
	Name: "MyAction",
	Params: []ActionParamSpec{
		{Name: "a", Type: ActionNumber},
		{Name: "b", Type: ActionNumber},
	},
}

var benchmarkThreeStringsSpec = ActionSpec{
	Name: "MyAction",
	Params: []ActionParamSpec{
		{Name: "a", Type: ActionString},
		{Name: "b", Type: ActionString},
		{Name: "c", Type: ActionString},
	},
}

func BenchmarkValidateActionParamsStringOnly(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"Juan"}}, nil)
	var name string

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &benchmarkStringOnlySpec, &name); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsNumberOnly(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"42"}}, nil)
	var age int

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &benchmarkNumberOnlySpec, &age); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsBooleanOnly(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"true"}}, nil)
	var enabled bool

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &benchmarkBooleanOnlySpec, &enabled); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsDateOnly(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"2026-07-05T12:34:56.123456789Z"}}, nil)
	var createdAt time.Time

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &benchmarkDateOnlySpec, &createdAt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsFileOnly(b *testing.B) {
	file := mustTestFile(b, "photo.jpg", "image/jpeg", "image-bytes")
	ctx := newActionContext("multipart/form-data; boundary=test", nil, map[string][]File{"0": {file}})
	var got File

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &benchmarkFileSpec, &got); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsJSONStruct(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {`{"name":"Juan","age":42}`}}, nil)
	var user actionUserPayload

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user = actionUserPayload{}
		if err := ValidateActionParams(ctx, &benchmarkJSONSpec, &user); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsJSONArray(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {`[{"name":"Juan","age":42},{"name":"Ana","age":30}]`}}, nil)
	var users []actionUserPayload

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		users = users[:0]
		if err := ValidateActionParams(ctx, &benchmarkJSONArraySpec, &users); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsTwoNumbers(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"42"}, "1": {"99"}}, nil)
	var a int
	var c int

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &benchmarkTwoNumbersSpec, &a, &c); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateActionParamsThreeStrings(b *testing.B) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"a"}, "1": {"b"}, "2": {"c"}}, nil)
	var a string
	var c string
	var d string

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateActionParams(ctx, &benchmarkThreeStringsSpec, &a, &c, &d); err != nil {
			b.Fatal(err)
		}
	}
}

var benchmarkFilesSpec = ActionSpec{
	Name:   "Upload",
	Params: []ActionParamSpec{{Name: "files", Type: ActionFiles}},
}

func BenchmarkValidateActionParamsFilesOnly(b *testing.B) {
	fileA := mustTestFile(b, "a.txt", "text/plain", "a")
	fileB := mustTestFile(b, "b.txt", "text/plain", "b")
	ctx := newActionContext("multipart/form-data; boundary=test", nil, map[string][]File{"0[]": {fileA, fileB}})
	var files []File

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		files = nil
		if err := ValidateActionParams(ctx, &benchmarkFilesSpec, &files); err != nil {
			b.Fatal(err)
		}
	}
}
