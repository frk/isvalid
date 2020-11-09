// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"errors"

	"github.com/frk/isvalid"
)

func init() {
	isvalid.RegisterRegexp(`foo`)
	isvalid.RegisterRegexp(`^[a-z]+\[[0-9]+\]$`)
	isvalid.RegisterRegexp(`\w+`)
}

func (v RegexpValidator) Validate() error {
	if !isvalid.Match(v.F1, `foo`) {
		return errors.New("F1 must match the regular expression: \"foo\"")
	}
	if v.F2 != nil && !isvalid.Match(*v.F2, `^[a-z]+\[[0-9]+\]$`) {
		return errors.New("F2 must match the regular expression: \"^[a-z]+\\\\[[0-9]+\\\\]$\"")
	}
	if v.F3 == nil || *v.F3 == nil || len(**v.F3) == 0 {
		return errors.New("F3 is required")
	} else if !isvalid.Match(**v.F3, `\w+`) {
		return errors.New("F3 must match the regular expression: \"\\\\w+\"")
	}
	return nil
}
