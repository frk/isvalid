// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"fmt"
)

func (v ReferencesValidator) Validate() error {
	if len(v.F1) > v.Max {
		return fmt.Errorf("F1 must be of length at most: %v", v.Max)
	}
	if v.F2 != nil {
		f := *v.F2
		if f < v.Min || f > v.Max {
			return fmt.Errorf("F2 must be between: %v and %v", v.Min, v.Max)
		}
	}
	return nil
}
