package cogni

import "fmt"

type cogniError struct{ msg string }

func (e *cogniError) Error() string { return e.msg }

func errRequired(msg string) error { return &cogniError{msg: msg} }

func errBadField(field string, value any, cause error) error {
	if cause != nil {
		return &cogniError{msg: fmt.Sprintf("%s: invalid value %v: %v", field, value, cause)}
	}
	return &cogniError{msg: fmt.Sprintf("%s: invalid value %v", field, value)}
}
