// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"errors"
)

func (v LengthValidator) Validate() error {
	if len(v.F1) != 10 {
		return errors.New("F1 must be of length: 10")
	}
	if v.F2 != nil && (len(*v.F2) < 8 || len(*v.F2) > 256) {
		return errors.New("F2 must be of length between: 8 and 256 (inclusive)")
	}
	if v.F3 == nil || *v.F3 == nil || len(**v.F3) == 0 {
		return errors.New("F3 is required")
	} else if len(**v.F3) < 1 || len(**v.F3) > 2 {
		return errors.New("F3 must be of length between: 1 and 2 (inclusive)")
	}
	if len(v.F4) < 4 {
		return errors.New("F4 must be of length at least: 4")
	}
	if len(v.F5) > 15 {
		return errors.New("F5 must be of length at most: 15")
	}
	return nil
}
