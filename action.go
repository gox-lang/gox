package gox

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Action interface{}

// gox:internal
const MaxJavaScriptSafeInteger int64 = 1<<53 - 1

// gox:internal
func ActionSignedIntegerType(value int64) string {
	if value < -MaxJavaScriptSafeInteger || value > MaxJavaScriptSafeInteger {
		return "bigint"
	}
	return "number"
}

// gox:internal
func ActionUnsignedIntegerType(value uint64) string {
	if value > uint64(MaxJavaScriptSafeInteger) {
		return "bigint"
	}
	return "number"
}

// gox:internal
type ActionHandler func(ctx *Context)

// gox:internal
type ActionParamType uint8

// gox:internal
const (
	ActionString ActionParamType = iota
	ActionNumber
	ActionBoolean
	ActionDate
	ActionFile
	ActionFiles
	ActionJSON
)

// gox:internal
type ActionSpec struct {
	Name   string
	Params []ActionParamSpec
}

// gox:internal
type ActionParamSpec struct {
	Name   string
	Type   ActionParamType
	JSName string
}

// gox:internal
func ValidateActionParams(ctx *Context, spec *ActionSpec, dest ...any) error {
	if ctx == nil || spec == nil {
		return fmt.Errorf("Action(): invalid action invocation")
	}
	ct := strings.ToLower(ctx.Request.ContentType())
	if !strings.Contains(ct, "multipart/form-data") && !strings.Contains(ct, "application/x-www-form-urlencoded") {
		if ct == "" {
			ct = "unknown"
		}
		return fmt.Errorf("%s(): unsupported Content-Type %q", spec.Name, ctx.Request.ContentType())
	}
	expected := len(spec.Params)
	if len(dest) != expected {
		return fmt.Errorf("%s(): expected %d arguments, received %d", spec.Name, expected, len(dest))
	}
	if received, ok := validateActionArgContract(ctx, spec); !ok {
		return fmt.Errorf("%s(): expected %d arguments, received %d", spec.Name, expected, received)
	}
	for i := range spec.Params {
		key := strconv.Itoa(i)
		param := spec.Params[i]
		if err := readActionParam(ctx, spec.Name, key, param, dest[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateActionArgContract(ctx *Context, spec *ActionSpec) (int, bool) {
	form := ctx.Request.Form.Fields.(collection[string])
	files := ctx.Request.Form.Files.(collection[File])
	expected := len(spec.Params)
	received := countReceivedActionArgs(form.values, files.values, spec)
	if received != expected {
		return received, false
	}

	for i, param := range spec.Params {
		key := strconv.Itoa(i)
		arrayKey := key + "[]"
		switch param.Type {
		case ActionFiles:
			if !hasActionArg(form.values, key) && !hasActionArg(form.values, arrayKey) && !hasActionArg(files.values, key) && !hasActionArg(files.values, arrayKey) {
				return received, false
			}
		case ActionFile:
			if !hasActionArg(form.values, key) && !hasActionArg(form.values, arrayKey) && !hasActionArg(files.values, key) && !hasActionArg(files.values, arrayKey) {
				return received, false
			}
		default:
			if !hasActionArg(form.values, key) && !hasActionArg(form.values, arrayKey) && !hasActionArg(files.values, key) && !hasActionArg(files.values, arrayKey) {
				return received, false
			}
		}
	}
	return received, true
}

func hasActionArg[T any](values map[string][]T, key string) bool {
	return len(values[key]) != 0
}

func countReceivedActionArgs(form map[string][]string, files map[string][]File, spec *ActionSpec) int {
	received := 0
	for _, items := range form {
		if len(items) != 0 {
			received += len(items)
		}
	}
	for key, items := range files {
		if len(items) == 0 {
			continue
		}
		if strings.HasSuffix(key, "[]") {
			idx, ok := actionArrayKeyIndex(key)
			if ok && idx >= 0 && idx < len(spec.Params) && spec.Params[idx].Type == ActionFiles {
				received++
				continue
			}
		}
		received += len(items)
	}
	return received
}

func actionArrayKeyIndex(key string) (int, bool) {
	if !strings.HasSuffix(key, "[]") {
		return 0, false
	}
	idx, err := strconv.Atoi(strings.TrimSuffix(key, "[]"))
	return idx, err == nil
}

func readActionParam(ctx *Context, action, key string, param ActionParamSpec, dest any) error {
	name := param.Name
	if name == "" {
		name = key
	}
	arrayKey := key + "[]"
	if param.Type == ActionFiles {
		if ctx.Request.Form.Fields.Has(key) || ctx.Request.Form.Fields.Has(arrayKey) {
			return actionTypeError(action, name, param, "string")
		}
		if ctx.Request.Form.Files.Has(key) {
			return actionTypeError(action, name, param, "File")
		}
		files := ctx.Request.Form.Files.(collection[File])
		items := files.values[arrayKey]
		if len(items) == 0 {
			return actionTypeError(action, name, param, "string")
		}
		return assignFiles(action, name, param, dest, items)
	}
	if param.Type == ActionFile {
		if ctx.Request.Form.Fields.Has(key) || ctx.Request.Form.Fields.Has(arrayKey) {
			return actionTypeError(action, name, param, "string")
		}
		if ctx.Request.Form.Files.Has(arrayKey) {
			return actionTypeError(action, name, param, "File[]")
		}
		f, ok := ctx.Request.Form.Files.Get(key)
		if !ok || f.IsZero() {
			return actionTypeError(action, name, param, "string")
		}

		return assignFile(action, name, param, dest, f)
	}
	if ctx.Request.Form.Files.Has(key) {
		return actionTypeError(action, name, param, "File")
	}
	if ctx.Request.Form.Files.Has(arrayKey) {
		return actionTypeError(action, name, param, "File[]")
	}
	text, ok := ctx.Request.Form.Fields.Get(key)
	if !ok {
		return actionTypeError(action, name, param, "string")
	}
	switch param.Type {
	case ActionString:
		return assignValue(action, name, param, dest, text)
	case ActionBoolean:
		v, err := strconv.ParseBool(text)
		if err != nil {
			return actionTypeError(action, name, param, "string")
		}
		return assignValue(action, name, param, dest, v)
	case ActionNumber:
		return assignNumber(action, name, param, dest, text)
	case ActionDate:
		v, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return actionTypeError(action, name, param, "string")
		}
		return assignValue(action, name, param, dest, v)
	case ActionJSON:
		if err := json.Unmarshal([]byte(text), dest); err != nil {
			return fmt.Errorf("%s(): argument %q contains invalid JSON", action, name)
		}
		return nil
	default:
		return actionTypeError(action, name, param, "unknown")
	}
}

func assignFiles(action, name string, param ActionParamSpec, dest any, files []File) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return actionTypeError(action, name, param, "File[]")
	}
	dv := rv.Elem()
	if dv.Kind() != reflect.Slice {
		return actionTypeError(action, name, param, "File[]")
	}
	fv := reflect.ValueOf(cloneSlice(files))
	if fv.Type().AssignableTo(dv.Type()) {
		dv.Set(fv)
		return nil
	}
	if fv.Type().ConvertibleTo(dv.Type()) {
		dv.Set(fv.Convert(dv.Type()))
		return nil
	}
	return actionTypeError(action, name, param, "File[]")
}

func assignFile(action, name string, param ActionParamSpec, dest any, f File) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return actionTypeError(action, name, param, "File")
	}
	dv := rv.Elem()
	fv := reflect.ValueOf(f)
	if fv.Type().AssignableTo(dv.Type()) {
		dv.Set(fv)
		return nil
	}
	if fv.Type().ConvertibleTo(dv.Type()) {
		dv.Set(fv.Convert(dv.Type()))
		return nil
	}
	if dv.Kind() == reflect.Slice && dv.Type().Elem().Kind() == reflect.Uint8 {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			return err
		}
		bv := reflect.ValueOf(b)
		if bv.Type().AssignableTo(dv.Type()) {
			dv.Set(bv)
		} else {
			dv.Set(bv.Convert(dv.Type()))
		}
		return nil
	}
	return actionTypeError(action, name, param, "File")
}

func assignNumber(action, name string, param ActionParamSpec, dest any, text string) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return actionTypeError(action, name, param, "string")
	}
	e := rv.Elem()
	switch e.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(text, 10, e.Type().Bits())
		if err != nil {
			return actionTypeError(action, name, param, "string")
		}
		e.SetInt(v)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v, err := strconv.ParseUint(text, 10, e.Type().Bits())
		if err != nil {
			return actionTypeError(action, name, param, "string")
		}
		e.SetUint(v)
		return nil
	case reflect.Float32, reflect.Float64:
		v, err := strconv.ParseFloat(text, e.Type().Bits())
		if err != nil {
			return actionTypeError(action, name, param, "string")
		}
		e.SetFloat(v)
		return nil
	default:
		return actionTypeError(action, name, param, "string")
	}
}

func assignValue(action, name string, param ActionParamSpec, dest any, value any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return actionTypeError(action, name, param, receivedJS(value))
	}
	dv := rv.Elem()
	vv := reflect.ValueOf(value)
	if !vv.Type().AssignableTo(dv.Type()) {
		if vv.Type().ConvertibleTo(dv.Type()) {
			vv = vv.Convert(dv.Type())
		} else {
			return actionTypeError(action, name, param, receivedJS(value))
		}
	}
	dv.Set(vv)
	return nil
}

func actionTypeError(action, name string, param ActionParamSpec, received string) error {
	return fmt.Errorf("%s(): argument %q expected %s, received %s", action, name, paramExpectedName(param), received)
}

func paramExpectedName(param ActionParamSpec) string {
	if param.JSName != "" {
		return param.JSName
	}
	switch param.Type {
	case ActionString:
		return "string"
	case ActionNumber:
		return "number"
	case ActionBoolean:
		return "boolean"
	case ActionDate:
		return "Date"
	case ActionFile:
		return "File"
	case ActionFiles:
		return "File[]"
	case ActionJSON:
		return "object"
	}
	return "unknown"
}

func receivedJS(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case time.Time:
		return "Date"
	case File:
		return "File"
	case []File:
		return "File[]"
	default:
		return "object"
	}
}
