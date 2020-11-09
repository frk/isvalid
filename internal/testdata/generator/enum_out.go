// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"errors"

	"github.com/frk/isvalid/internal/testdata/mypkg"
)

func (v EnumValidator) Validate() error {
	if v.F1 != myenum0 && v.F1 != myenum1 && v.F1 != myenum2 && v.F1 != myenum4 && v.F1 != myenum6 {
		return errors.New("F1 is not valid")
	}
	if v.F2 != mypkg.MyFoo && v.F2 != mypkg.MyBar && v.F2 != mypkg.MyBaz {
		return errors.New("F2 is not valid")
	}
	if v.F3 == nil || *v.F3 == nil || len(**v.F3) == 0 {
		return errors.New("F3 is required")
	} else {
		f := **v.F3
		if len(f) != 3 {
			return errors.New("F3 must be of length: 3")
		} else if f != gibfoo && f != gibbar && f != gibbaz && f != gibquux {
			return errors.New("F3 is not valid")
		}
	}
	return nil
}
