// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"errors"
	"strings"

	"github.com/frk/isvalid"
)

func (v NestedFieldsWithRulesAndNilGuardValidator) Validate() error {
	if v.G2.F1 != nil && !isvalid.Email(*v.G2.F1) {
		return errors.New("G2.F1 must be a valid email address")
	}
	if v.G2.F2 != nil && *v.G2.F2 != nil && !isvalid.Email(**v.G2.F2) {
		return errors.New("G2.F2 must be a valid email address")
	}
	if v.G2.G3 != nil {
		f := *v.G2.G3
		if f.F3 != nil && *f.F3 != nil && **f.F3 != nil {
			f := ***f.F3
			if !isvalid.Hex(f) {
				return errors.New("G2.G3.F3 must be a valid hexadecimal string")
			} else if len(f) < 8 || len(f) > 128 {
				return errors.New("G2.G3.F3 must be of length between: 8 and 128 (inclusive)")
			}
		}
	}
	if v.G2.G4 != nil && *v.G2.G4 != nil && **v.G2.G4 != nil {
		f := ***v.G2.G4
		if f.G5 != nil && *f.G5 != nil {
			f := **f.G5
			if f.F3 == nil || *f.F3 == nil || len(**f.F3) == 0 {
				return errors.New("G2.G4.G5.F3 is required")
			} else if !strings.HasPrefix(**f.F3, "foo") {
				return errors.New("G2.G4.G5.F3 must be prefixed with: \"foo\"")
			} else if !strings.Contains(**f.F3, "bar") {
				return errors.New("G2.G4.G5.F3 must contain substring: \"bar\"")
			} else if !strings.HasSuffix(**f.F3, "baz") && !strings.HasSuffix(**f.F3, "quux") {
				return errors.New("G2.G4.G5.F3 must be suffixed with: \"baz\" or \"quux\"")
			} else if len(**f.F3) < 8 || len(**f.F3) > 64 {
				return errors.New("G2.G4.G5.F3 must be of length between: 8 and 64 (inclusive)")
			}
		}
	}
	return nil
}
