// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"errors"

	"github.com/frk/isvalid"
)

func (v EINValidator) Validate() error {
	if !isvalid.EIN(v.F1) {
		return errors.New("F1 must be a valid EIN")
	}
	if v.F2 != nil && *v.F2 != nil {
		f := **v.F2
		if !isvalid.EIN(f) {
			return errors.New("F2 must be a valid EIN")
		}
	}
	if v.F3 == nil || *v.F3 == nil || len(**v.F3) == 0 {
		return errors.New("F3 is required")
	} else {
		f := **v.F3
		if !isvalid.EIN(f) {
			return errors.New("F3 must be a valid EIN")
		}
	}
	return nil
}