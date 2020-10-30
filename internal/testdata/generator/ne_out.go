// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"errors"
)

func (v NotEqualsValidator) Validate() error {
	if v.F1 == "foo" {
		return errors.New("F1 must not be equal to: \"foo\"")
	}
	if v.F2 != nil {
		f := *v.F2
		if f == 123 || f == 0 || f == 321 {
			return errors.New("F2 must not be equal to: 123 or 0 or 321")
		}
	}
	if v.F3 == nil || *v.F3 == nil || **v.F3 == nil {
		return errors.New("F3 is required")
	} else {
		f := **v.F3
		if f == "foo" || f == 123 || f == false || f == 3.14 {
			return errors.New("F3 must not be equal to: \"foo\" or 123 or false or 3.14")
		}
	}
	return nil
}
