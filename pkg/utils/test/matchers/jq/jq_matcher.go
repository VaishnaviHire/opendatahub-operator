package jq

import (
	"fmt"

	"github.com/itchyny/gojq"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

//nolint:ireturn
func Match(format string, args ...any) types.GomegaMatcher {
	return &jqMatcher{
		Expression: fmt.Sprintf(format, args...),
	}
}

var _ types.GomegaMatcher = &jqMatcher{}

type jqMatcher struct {
	Expression       string
	firstFailurePath []interface{}
}

func (matcher *jqMatcher) Match(actual interface{}) (bool, error) {
	query, err := gojq.Parse(matcher.Expression)
	if err != nil {
		return false, fmt.Errorf("unable to parse expression %s, %w", matcher.Expression, err)
	}

	data, err := toType(actual)
	if err != nil {
		return false, err
	}

	it := query.Run(data)

	v, ok := it.Next()
	if !ok {
		return false, nil
	}

	if err, ok := v.(error); ok {
		return false, err
	}

	if match, ok := v.(bool); ok {
		return match, nil
	}

	return false, nil
}

func (matcher *jqMatcher) FailureMessage(actual interface{}) string {
	return formattedMessage(format.Message(fmt.Sprintf("%v", actual), "to match expression", matcher.Expression), matcher.firstFailurePath)
}

func (matcher *jqMatcher) NegatedFailureMessage(actual interface{}) string {
	return formattedMessage(format.Message(fmt.Sprintf("%v", actual), "not to match expression", matcher.Expression), matcher.firstFailurePath)
}
