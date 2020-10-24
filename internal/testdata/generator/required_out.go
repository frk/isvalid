// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"errors"
)

func (v RequiredValidator) Validate() error {
	if v.F1 == nil || len(*v.F1) == 0 {
		return errors.New("F1 is required")
	}
	if len(v.F2) == 0 {
		return errors.New("F2 is required")
	}
	if v.F3 == 0 {
		return errors.New("F3 is required")
	}
	if v.F4 == nil || *v.F4 == 0.0 {
		return errors.New("F4 is required")
	}
	if v.F5 == nil || *v.F5 == nil {
		return errors.New("F5 is required")
	}
	if v.F6 == nil || *v.F6 == nil || **v.F6 == false {
		return errors.New("F6 is required")
	}
	if len(v.F7) == 0 {
		return errors.New("F7 is required")
	}
	if v.F8 == nil || len(*v.F8) == 0 {
		return errors.New("F8 is required")
	}
	{
		w := v.G1
		if w.F1 == nil || len(*w.F1) == 0 {
			return errors.New("F1-1 is required")
		}
		if w := w.G2; w == nil {
			return errors.New("G2 is required")
		} else {
			if w.F1 == nil || len(*w.F1) == 0 {
				return errors.New("F1-2 is required")
			}
		}
		if w.F2 == nil || len(*w.F2) == 0 {
			return errors.New("F2-1 is required")
		}
	}
	if v.FX == nil || *v.FX == nil || **v.FX == nil || ***v.FX == nil || ****v.FX == nil || *****v.FX == nil {
		return errors.New("FX is required")
	}
	if v.GX == nil || *v.GX == nil || **v.GX == nil || ***v.GX == nil || ****v.GX == nil {
		return errors.New("GX is required")
	} else {
		w := ****v.GX
		if len(w.F1) == 0 {
			return errors.New("F1-3 is required")
		}
		if w.F2 == 0 {
			return errors.New("F2-2 is required")
		}
		if w.F3 == 0.0 {
			return errors.New("F3-1 is required")
		}
	}
	return nil
}