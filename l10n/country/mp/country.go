package mp

import (
	"regexp"

	"github.com/frk/isvalid/l10n/country"
)

func init() {
	country.Add(country.Country{
		A2: "MP", A3: "MNP", Num: "580",
		Zip: regexp.MustCompile(`^[0-9]{5}(?:-[0-9]{4})?$`),
	})
}
