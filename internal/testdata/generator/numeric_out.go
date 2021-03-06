// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"errors"

	"github.com/frk/isvalid"
)

func (v NumericValidator) Validate() error {
	if !isvalid.Numeric(v.F1) {
		return errors.New("F1 string content must match a numeric value")
	}
	if v.F2 != nil && *v.F2 != nil && !isvalid.Numeric(**v.F2) {
		return errors.New("F2 string content must match a numeric value")
	}
	if v.F3 == nil || *v.F3 == nil || len(**v.F3) == 0 {
		return errors.New("F3 is required")
	} else if !isvalid.Numeric(**v.F3) {
		return errors.New("F3 string content must match a numeric value")
	}
	return nil
}
