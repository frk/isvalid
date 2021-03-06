package cz

import (
	"regexp"

	"github.com/frk/isvalid/l10n/country"
)

func init() {
	country.Add(country.Country{
		A2: "CZ", A3: "CZE", Num: "203",
		Zip:   regexp.MustCompile(`^[0-9]{3}[ ]?[0-9]{2}$`),
		Phone: regexp.MustCompile(`^(?:\+?420)?[ ]?[1-9][0-9]{2}[ ]?[0-9]{3}[ ]?[0-9]{3}$`),
		// 8, 9 or 10 characters -- i.e. CZ12345678, CZ123456789, CZ1234567890
		VAT: regexp.MustCompile(`^CZ[0-9]{8,10}$`),
	})
}
