package test

type ExpectedActionErrorRule interface {
	MatchActionError(value any) ActionValueMatchResult
}

type ExpectedActionResultRule interface {
	MatchActionResult(value any) ActionValueMatchResult
}

type ExpectedActionErrorNil struct{}
type ExpectedActionErrorExact struct {
	Value any `json:"value"`
}

func (ExpectedActionErrorNil) MatchActionError(value any) ActionValueMatchResult {
	return ActionValueMatchResult{
		Rule:     "nil",
		Expected: nil,
		Response: value,
		Match:    value == nil,
	}
}

func (r ExpectedActionErrorExact) MatchActionError(value any) ActionValueMatchResult {
	return ActionValueMatchResult{
		Rule:     "exact",
		Expected: r.Value,
		Response: value,
		Match:    valuesEqual(value, r.Value),
	}
}

type expectedActionErrorFactory struct{}

var ExpectedActionError expectedActionErrorFactory

func (expectedActionErrorFactory) Nil() ExpectedActionErrorNil {
	return ExpectedActionErrorNil{}
}

func (expectedActionErrorFactory) Exact(value any) ExpectedActionErrorExact {
	return ExpectedActionErrorExact{Value: value}
}

type ExpectedActionResultExact struct {
	Value any `json:"value"`
}

type ExpectedActionResultPartial map[string]any

func (r ExpectedActionResultExact) MatchActionResult(value any) ActionValueMatchResult {
	match := valuesEqual(value, r.Value)
	result := ActionValueMatchResult{
		Rule:     "exact",
		Expected: fileDisplayValue(r.Value),
		Response: fileDisplayValue(value),
		Match:    match,
	}
	if !match {
		result.JSONFindings = []JSONMatchFinding{{Path: "$", Expected: fileDisplayValue(r.Value), Response: fileDisplayValue(value), Found: true, Match: false, Message: "action result does not equal expected value"}}
	}
	return result
}

func (r ExpectedActionResultPartial) MatchActionResult(value any) ActionValueMatchResult {
	expected := map[string]any(r)
	findings := jsonContainsFindings(value, expected)
	match := jsonFindingsMatch(findings)
	fields := make(map[string]MatchValue, len(r))
	actual, _ := normalizeJSONValue(value).(map[string]any)
	for key, expectedValue := range expected {
		response, exists := actual[key]
		fieldMatch := exists && jsonContains(response, expectedValue)
		message := ""
		if !fieldMatch {
			message = "missing expected action result field"
		}
		fields[key] = MatchValue{
			Expected: expectedValue,
			Response: response,
			Match:    fieldMatch,
			Rule:     "partial",
			Message:  message,
		}
	}
	message := ""
	if !match {
		message = "missing expected action result subset"
	}
	return ActionValueMatchResult{
		Rule:         "partial",
		Expected:     expected,
		Response:     value,
		Match:        match,
		Fields:       fields,
		JSONFindings: findings,
		Message:      message,
	}
}

type expectedActionResultFactory struct{}

var ExpectedActionResult expectedActionResultFactory

func (expectedActionResultFactory) Exact(value any) ExpectedActionResultExact {
	return ExpectedActionResultExact{Value: value}
}

func (expectedActionResultFactory) Partial(values map[string]any) ExpectedActionResultPartial {
	return ExpectedActionResultPartial(values)
}
