package test

import (
	"fmt"
	"math"
	"reflect"
	"strings"
)

func matchStringMap(rule string, expected Map, response map[string]string, strict bool, exclude bool) HeaderMatchResult {
	fields := make(map[string]MatchValue, len(expected))
	match := true

	for key, expectedAny := range expected {
		expectedValue := stringifyExpectedMapValue(expectedAny)
		canonical := canonicalKey(key)
		responseValue, exists := response[canonical]

		fieldMatch := false
		if exclude {
			if expectedValue == "" {
				fieldMatch = !exists
			} else {
				fieldMatch = !exists || responseValue != expectedValue
			}
		} else {
			if expectedValue == "" {
				fieldMatch = exists
			} else {
				fieldMatch = exists && responseValue == expectedValue
			}
		}

		message := ""
		if !fieldMatch {
			match = false
			message = stringMapMismatchMessage(rule, exists, expectedValue, responseValue, exclude)
		}

		fields[key] = MatchValue{
			Expected: expectedValue,
			Response: responseValue,
			Match:    fieldMatch,
			Rule:     rule,
			Message:  message,
		}
	}

	if strict {
		for key, responseValue := range response {
			found := false
			for expectedKey := range expected {
				if canonicalKey(expectedKey) == key {
					found = true
					break
				}
			}
			if !found {
				match = false
				fields[key] = MatchValue{
					Expected: "",
					Response: responseValue,
					Match:    false,
					Rule:     "equal-extra",
					Message:  "unexpected extra field",
				}
			}
		}
	}

	return HeaderMatchResult{
		Rule:   rule,
		Match:  match,
		Fields: fields,
	}
}

func stringifyExpectedMapValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func stringMapMismatchMessage(rule string, exists bool, expected string, response string, exclude bool) string {
	if exclude {
		if expected == "" && exists {
			return "field must not exist"
		}
		if exists && response == expected {
			return "field has forbidden value"
		}
		return "field violates notContains rule"
	}
	if !exists {
		return "field is missing"
	}
	if expected != "" && response != expected {
		return fmt.Sprintf("field value mismatch: expected %q, got %q", expected, response)
	}
	if rule == "equal" {
		return "field set is not equal"
	}
	return "field mismatch"
}

func matchStringCustomMap(rule string, expected map[string]ExpectedStringRule, response map[string]string) HeaderMatchResult {
	fields := make(map[string]MatchValue, len(expected))
	match := true

	for key, expectedRule := range expected {
		canonical := canonicalKey(key)
		responseValue, exists := response[canonical]

		var field MatchValue
		if expectedRule == nil {
			field = MatchValue{Rule: rule, Expected: nil, Response: responseValue, Match: false, Message: "custom rule is nil"}
		} else {
			field = expectedRule.MatchString(responseValue, exists)
		}
		if !field.Match {
			match = false
		}
		fields[key] = field
	}

	return HeaderMatchResult{Rule: rule, Match: match, Fields: fields}
}

func valuesEqual(a, b any) bool {
	if equal, handled := fileValuesEqual(a, b); handled {
		return equal
	}
	if equal, handled := fileValuesEqual(b, a); handled {
		return equal
	}
	if numericValuesEqual(a, b) {
		return true
	}
	return reflect.DeepEqual(normalizeJSONValue(a), normalizeJSONValue(b))
}

func numericValuesEqual(a, b any) bool {
	af, aok := numericFloat64(a)
	bf, bok := numericFloat64(b)
	if !aok || !bok {
		return false
	}
	return math.Abs(af-bf) == 0
}

func numericFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}

func normalizeJSONValue(value any) any {
	switch v := value.(type) {
	case float64:
		if v == float64(int64(v)) {
			return int64(v)
		}
		return v
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return uint64(v)
	case uint8:
		return uint64(v)
	case uint16:
		return uint64(v)
	case uint32:
		return uint64(v)
	case uint64:
		return v
	case Map:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = normalizeJSONValue(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = normalizeJSONValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeJSONValue(item)
		}
		return out
	case []Map:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeJSONValue(item)
		}
		return out
	default:
		return value
	}
}

func jsonContains(actual any, expected any) bool {
	actual = normalizeJSONValue(actual)
	expected = normalizeJSONValue(expected)

	switch exp := expected.(type) {
	case map[string]any:
		act, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for key, expectedValue := range exp {
			actualValue, exists := act[key]
			if !exists || !jsonContains(actualValue, expectedValue) {
				return false
			}
		}
		return true
	case []any:
		act, ok := actual.([]any)
		if !ok {
			return false
		}
		for _, expectedItem := range exp {
			found := false
			for _, actualItem := range act {
				if jsonContains(actualItem, expectedItem) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	default:
		return valuesEqual(actual, expected)
	}
}

func jsonPath(value any, path string) (any, bool) {
	if path == "" {
		return value, true
	}

	current := value
	parts := strings.Split(path, ".")
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
