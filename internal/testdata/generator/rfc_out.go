// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"errors"

	"github.com/frk/isvalid"
)

func (v RFCValidator) Validate() error {
	if !isvalid.RFC(v.F1, 1234) {
		return errors.New("F1 must be a valid RFC 1234")
	}
	if v.F2 != nil && *v.F2 != nil {
		f := **v.F2
		if !isvalid.RFC(f, 4321) {
			return errors.New("F2 must be a valid RFC 4321")
		}
	}
	if v.F3 == nil || *v.F3 == nil || len(**v.F3) == 0 {
		return errors.New("F3 is required")
	} else {
		f := **v.F3
		if !isvalid.RFC(f, 6) {
			return errors.New("F3 must be a valid RFC 6")
		}
	}
	return nil
}
