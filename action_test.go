package gox

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

type actionUserPayload struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type actionAliasName string
type actionAliasAge int

type actionAliasFlag bool

var testActionSpec = ActionSpec{
	Name: "MyAction",
	Params: []ActionParamSpec{
		{Name: "name", Type: ActionString},
		{Name: "age", Type: ActionNumber},
	},
}

func newActionContext(contentType string, form map[string][]string, files map[string][]File) *Context {
	return &Context{Request: NewRequest(RequestInput{
		ContentType: contentType,
		Form:        NewCollection[string](form),
		Files:       NewCollection[File](files),
	})}
}

func mustTestFile(t testing.TB, name, contentType, data string) File {
	t.Helper()
	f, err := NewFileWithContentType(name, contentType, strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func requireActionError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %q", want)
	}
	if got := err.Error(); got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestValidateActionParamsURLencodedSuccess(t *testing.T) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"Juan"}, "1": {"42"}}, nil)
	var name string
	var age int

	if err := ValidateActionParams(ctx, &testActionSpec, &name, &age); err != nil {
		t.Fatal(err)
	}
	if name != "Juan" || age != 42 {
		t.Fatalf("decoded name=%q age=%d", name, age)
	}
}

func TestValidateActionParamsMultipartSuccess(t *testing.T) {
	ctx := newActionContext("multipart/form-data; boundary=test", map[string][]string{"0": {"Juan"}, "1": {"42"}}, nil)
	var name string
	var age int

	if err := ValidateActionParams(ctx, &testActionSpec, &name, &age); err != nil {
		t.Fatal(err)
	}
	if name != "Juan" || age != 42 {
		t.Fatalf("decoded name=%q age=%d", name, age)
	}
}

func TestValidateActionParamsUnsupportedContentType(t *testing.T) {
	ctx := newActionContext("application/json", map[string][]string{"0": {"Juan"}, "1": {"42"}}, nil)
	var name string
	var age int

	err := ValidateActionParams(ctx, &testActionSpec, &name, &age)
	requireActionError(t, err, `MyAction(): unsupported Content-Type "application/json"`)
}

func TestValidateActionParamsFewerArguments(t *testing.T) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"Juan"}}, nil)
	var name string
	var age int

	err := ValidateActionParams(ctx, &testActionSpec, &name, &age)
	requireActionError(t, err, "MyAction(): expected 2 arguments, received 1")
}

func TestValidateActionParamsMoreArguments(t *testing.T) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"Juan"}, "1": {"42"}, "2": {"extra"}}, nil)
	var name string
	var age int

	err := ValidateActionParams(ctx, &testActionSpec, &name, &age)
	requireActionError(t, err, "MyAction(): expected 2 arguments, received 3")
}

func TestValidateActionParamsMissingIndex(t *testing.T) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"Juan"}, "2": {"42"}}, nil)
	var name string
	var age int

	err := ValidateActionParams(ctx, &testActionSpec, &name, &age)
	requireActionError(t, err, "MyAction(): expected 2 arguments, received 2")
}

func TestValidateActionParamsInvalidKey(t *testing.T) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"Juan"}, "foo": {"42"}}, nil)
	var name string
	var age int

	err := ValidateActionParams(ctx, &testActionSpec, &name, &age)
	requireActionError(t, err, "MyAction(): expected 2 arguments, received 2")
}

func TestValidateActionParamsDuplicateKey(t *testing.T) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"Juan", "Pedro"}, "1": {"42"}}, nil)
	var name string
	var age int

	err := ValidateActionParams(ctx, &testActionSpec, &name, &age)
	requireActionError(t, err, "MyAction(): expected 2 arguments, received 3")
}

func TestValidateActionParamsNumberTypeError(t *testing.T) {
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"Juan"}, "1": {"abc"}}, nil)
	var name string
	var age int

	err := ValidateActionParams(ctx, &testActionSpec, &name, &age)
	requireActionError(t, err, `MyAction(): argument "age" expected number, received string`)
}

func TestValidateActionParamsBooleanSuccessAndTypeError(t *testing.T) {
	spec := ActionSpec{Name: "MyAction", Params: []ActionParamSpec{{Name: "enabled", Type: ActionBoolean}}}

	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"true"}}, nil)
	var enabled bool
	if err := ValidateActionParams(ctx, &spec, &enabled); err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("enabled = false, want true")
	}

	ctx = newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"yes"}}, nil)
	err := ValidateActionParams(ctx, &spec, &enabled)
	requireActionError(t, err, `MyAction(): argument "enabled" expected boolean, received string`)
}

func TestValidateActionParamsDateSuccessAndTypeError(t *testing.T) {
	spec := ActionSpec{Name: "MyAction", Params: []ActionParamSpec{{Name: "createdAt", Type: ActionDate}}}
	input := "2026-07-05T12:34:56.123456789Z"
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {input}}, nil)
	var createdAt time.Time

	if err := ValidateActionParams(ctx, &spec, &createdAt); err != nil {
		t.Fatal(err)
	}
	if got := createdAt.Format(time.RFC3339Nano); got != input {
		t.Fatalf("createdAt = %q, want %q", got, input)
	}

	ctx = newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"not-a-date"}}, nil)
	err := ValidateActionParams(ctx, &spec, &createdAt)
	requireActionError(t, err, `MyAction(): argument "createdAt" expected Date, received string`)
}

func TestValidateActionParamsJSONSuccess(t *testing.T) {
	spec := ActionSpec{Name: "MyAction", Params: []ActionParamSpec{{Name: "user", Type: ActionJSON, JSName: "User"}}}
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {`{"name":"Juan","age":42}`}}, nil)
	var user actionUserPayload

	if err := ValidateActionParams(ctx, &spec, &user); err != nil {
		t.Fatal(err)
	}
	if user.Name != "Juan" || user.Age != 42 {
		t.Fatalf("decoded user = %#v", user)
	}
}

func TestValidateActionParamsJSONInvalid(t *testing.T) {
	spec := ActionSpec{Name: "MyAction", Params: []ActionParamSpec{{Name: "user", Type: ActionJSON, JSName: "User"}}}
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {`{"name":`}}, nil)
	var user actionUserPayload

	err := ValidateActionParams(ctx, &spec, &user)
	requireActionError(t, err, `MyAction(): argument "user" contains invalid JSON`)
}

func TestValidateActionParamsFileSuccess(t *testing.T) {
	spec := ActionSpec{Name: "MyAction", Params: []ActionParamSpec{{Name: "photo", Type: ActionFile}}}
	file := mustTestFile(t, "photo.jpg", "image/jpeg", "image-bytes")
	ctx := newActionContext("multipart/form-data; boundary=test", nil, map[string][]File{"0": {file}})
	var got File

	if err := ValidateActionParams(ctx, &spec, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name() != "photo.jpg" || got.ContentType() != "image/jpeg" || got.Len() != int64(len("image-bytes")) {
		t.Fatalf("decoded file name=%q type=%q len=%d", got.Name(), got.ContentType(), got.Len())
	}
}

func TestValidateActionParamsFileToBytesSuccess(t *testing.T) {
	spec := ActionSpec{Name: "MyAction", Params: []ActionParamSpec{{Name: "photo", Type: ActionFile}}}
	file := mustTestFile(t, "photo.txt", "text/plain", "hello")
	ctx := newActionContext("multipart/form-data; boundary=test", nil, map[string][]File{"0": {file}})
	var data []byte

	if err := ValidateActionParams(ctx, &spec, &data); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, []byte("hello")) {
		t.Fatalf("data = %q, want hello", string(data))
	}
}

func TestValidateActionParamsFileSentAsString(t *testing.T) {
	spec := ActionSpec{Name: "MyAction", Params: []ActionParamSpec{{Name: "photo", Type: ActionFile}}}
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"not-a-file"}}, nil)
	var file File

	err := ValidateActionParams(ctx, &spec, &file)
	requireActionError(t, err, `MyAction(): argument "photo" expected File, received string`)
}

func TestValidateActionParamsStringSentAsFile(t *testing.T) {
	spec := ActionSpec{Name: "MyAction", Params: []ActionParamSpec{{Name: "name", Type: ActionString}}}
	file := mustTestFile(t, "name.txt", "text/plain", "Juan")
	ctx := newActionContext("multipart/form-data; boundary=test", nil, map[string][]File{"0": {file}})
	var name string

	err := ValidateActionParams(ctx, &spec, &name)
	requireActionError(t, err, `MyAction(): argument "name" expected string, received File`)
}

func TestValidateActionParamsAliases(t *testing.T) {
	spec := ActionSpec{Name: "MyAction", Params: []ActionParamSpec{
		{Name: "name", Type: ActionString},
		{Name: "age", Type: ActionNumber},
		{Name: "enabled", Type: ActionBoolean},
	}}
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"Juan"}, "1": {"42"}, "2": {"true"}}, nil)
	var name actionAliasName
	var age actionAliasAge
	var enabled actionAliasFlag

	if err := ValidateActionParams(ctx, &spec, &name, &age, &enabled); err != nil {
		t.Fatal(err)
	}
	if name != "Juan" || age != 42 || !enabled {
		t.Fatalf("decoded aliases name=%q age=%d enabled=%v", name, age, enabled)
	}
}

type actionAliasFiles []File

func TestValidateActionParamsFilesSuccess(t *testing.T) {
	spec := ActionSpec{Name: "Upload", Params: []ActionParamSpec{
		{Name: "title", Type: ActionString},
		{Name: "files", Type: ActionFiles},
	}}
	fileA := mustTestFile(t, "a.txt", "text/plain", "a")
	fileB := mustTestFile(t, "b.txt", "text/plain", "b")
	ctx := newActionContext("multipart/form-data; boundary=test", map[string][]string{"0": {"Gallery"}}, map[string][]File{"1[]": {fileA, fileB}})
	var title string
	var files []File

	if err := ValidateActionParams(ctx, &spec, &title, &files); err != nil {
		t.Fatal(err)
	}
	if title != "Gallery" || len(files) != 2 || files[0].Name() != "a.txt" || files[1].Name() != "b.txt" {
		t.Fatalf("decoded title=%q files=%d", title, len(files))
	}
}

func TestValidateActionParamsFilesAliasSuccess(t *testing.T) {
	spec := ActionSpec{Name: "Upload", Params: []ActionParamSpec{{Name: "files", Type: ActionFiles}}}
	fileA := mustTestFile(t, "a.txt", "text/plain", "a")
	fileB := mustTestFile(t, "b.txt", "text/plain", "b")
	ctx := newActionContext("multipart/form-data; boundary=test", nil, map[string][]File{"0[]": {fileA, fileB}})
	var files actionAliasFiles

	if err := ValidateActionParams(ctx, &spec, &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Name() != "a.txt" || files[1].Name() != "b.txt" {
		t.Fatalf("decoded files=%d", len(files))
	}
}

func TestValidateActionParamsFilesCountAsOneArgument(t *testing.T) {
	spec := ActionSpec{Name: "Upload", Params: []ActionParamSpec{
		{Name: "title", Type: ActionString},
		{Name: "files", Type: ActionFiles},
	}}
	fileA := mustTestFile(t, "a.txt", "text/plain", "a")
	fileB := mustTestFile(t, "b.txt", "text/plain", "b")
	fileC := mustTestFile(t, "c.txt", "text/plain", "c")
	ctx := newActionContext("multipart/form-data; boundary=test", map[string][]string{"0": {"Gallery"}}, map[string][]File{"1[]": {fileA, fileB, fileC}})
	var title string
	var files []File

	if err := ValidateActionParams(ctx, &spec, &title, &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("files=%d, want 3", len(files))
	}
}

func TestValidateActionParamsFilesRejectsSingleFileKey(t *testing.T) {
	spec := ActionSpec{Name: "Upload", Params: []ActionParamSpec{{Name: "files", Type: ActionFiles}}}
	file := mustTestFile(t, "a.txt", "text/plain", "a")
	ctx := newActionContext("multipart/form-data; boundary=test", nil, map[string][]File{"0": {file}})
	var files []File

	err := ValidateActionParams(ctx, &spec, &files)
	requireActionError(t, err, `Upload(): argument "files" expected File[], received File`)
}

func TestValidateActionParamsFileRejectsArrayKey(t *testing.T) {
	spec := ActionSpec{Name: "Upload", Params: []ActionParamSpec{{Name: "file", Type: ActionFile}}}
	fileA := mustTestFile(t, "a.txt", "text/plain", "a")
	fileB := mustTestFile(t, "b.txt", "text/plain", "b")
	ctx := newActionContext("multipart/form-data; boundary=test", nil, map[string][]File{"0[]": {fileA, fileB}})
	var file File

	err := ValidateActionParams(ctx, &spec, &file)
	requireActionError(t, err, "Upload(): expected 1 arguments, received 2")
}

func TestValidateActionParamsFilesSentAsString(t *testing.T) {
	spec := ActionSpec{Name: "Upload", Params: []ActionParamSpec{{Name: "files", Type: ActionFiles}}}
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0[]": {"not-a-file"}}, nil)
	var files []File

	err := ValidateActionParams(ctx, &spec, &files)
	requireActionError(t, err, `Upload(): argument "files" expected File[], received string`)
}

func TestValidateActionParamsFilesSentAsStringKey(t *testing.T) {
	spec := ActionSpec{Name: "Upload", Params: []ActionParamSpec{{Name: "files", Type: ActionFiles}}}
	ctx := newActionContext("application/x-www-form-urlencoded", map[string][]string{"0": {"not-a-file"}}, nil)
	var files []File

	err := ValidateActionParams(ctx, &spec, &files)
	requireActionError(t, err, `Upload(): argument "files" expected File[], received string`)
}
