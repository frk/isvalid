// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"errors"
)

func (v MaxValidator) Validate() error {
	if v.F1 > 3.14 {
		return errors.New("F1 must be less than or equal to: 3.14")
	}
	if v.F2 != nil {
		f := *v.F2
		if f > 123 {
			return errors.New("F2 must be less than or equal to: 123")
		}
	}
	if v.F3 == nil || *v.F3 == nil || **v.F3 == 0 {
		return errors.New("F3 is required")
	} else {
		f := **v.F3
		if f > 1 {
			return errors.New("F3 must be less than or equal to: 1")
		}
	}
	return nil
}
