// DO NOT EDIT. This file was generated by "github.com/frk/isvalid".

package testdata

import (
	"errors"

	"github.com/frk/isvalid"
)

func (v FQDNValidator) Validate() error {
	if !isvalid.FQDN(v.F1) {
		return errors.New("F1 must be a valid FQDN")
	}
	if !isvalid.FQDN(v.F2) {
		return errors.New("F2 must be a valid FQDN")
	}
	if v.F3 != nil && !isvalid.FQDN(*v.F3) {
		return errors.New("F3 must be a valid FQDN")
	}
	if v.F4 != nil && !isvalid.FQDN(*v.F4) {
		return errors.New("F4 must be a valid FQDN")
	}
	if v.F5 != nil && !isvalid.FQDN(*v.F5) {
		return errors.New("F5 must be a valid FQDN")
	}
	return nil
}