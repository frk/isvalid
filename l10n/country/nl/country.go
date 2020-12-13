package nl

import (
	"regexp"

	"github.com/frk/isvalid/l10n/country"
)

func init() {
	country.Add(country.Country{
		A2: "NL", A3: "NLD", Num: "528",
		Zip:   regexp.MustCompile(`^(?i)[0-9]{4}\s?[a-z]{2}$`),
		Phone: regexp.MustCompile(`^(?:(?:(?:\+|00)?31\(0\))|(?:(?:\+|00)?31)|0)6{1}[0-9]{8}$`),
	})
}
