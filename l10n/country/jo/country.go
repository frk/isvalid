package jo

import (
	"regexp"

	"github.com/frk/isvalid/l10n/country"
)

func init() {
	country.Add(country.Country{
		A2: "JO", A3: "JOR", Num: "400",
		Zip:   country.RxZip5Digits,
		Phone: regexp.MustCompile(`^(?:\+?962|0)?7[789][0-9]{7}$`),
	})
}
