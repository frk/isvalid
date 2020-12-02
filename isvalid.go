package isvalid

import (
	"encoding/base64"
	"net"
	"net/mail"
	"regexp"
	"strconv"
	"strings"

	"github.com/frk/isvalid/locale"
)

var rxASCII = regexp.MustCompile(`^[[:ascii:]]*$`)

// ASCII reports whether or not v is an ASCII string.
//
// isvalid:rule
//	{ "name": "ascii", "err": { "text": "must contain only ASCII characters" } }
func ASCII(v string) bool {
	return rxASCII.MatchString(v)
}

// Alpha reports whether or not v is a valid alphabetic string.
//
// isvalid:rule
//	{
//		"name": "alpha",
//		"opts": [[ { "key": null, "value": "en" } ]],
//		"err": { "text": "must be an alphabetic string" }
//	}
func Alpha(v string, loc ...string) bool {
	// TODO
	return false
}

// Alnum reports whether or not v is a valid alphanumeric string.
//
// isvalid:rule
//	{
//		"name": "alnum",
//		"opts": [[ { "key": null, "value": "en" } ]],
//		"err": { "text": "must be an alphanumeric string" }
//	}
func Alnum(v string, loc ...string) bool {
	// TODO
	return false
}

var rxBIC = regexp.MustCompile(`^[A-z]{4}[A-z]{2}\w{2}(\w{3})?$`)

// BIC reports whether or not v represents a valid Bank Identification Code or SWIFT code.
//
// isvalid:rule
//	{ "name": "bic", "err": { "text": "must be a valid BIC or SWIFT code" } }
func BIC(v string) bool {
	return rxBIC.MatchString(v)
}

var rxBTC = regexp.MustCompile(`^(bc1|[13])[a-zA-HJ-NP-Z0-9]{25,39}$`)

// BTC reports whether or not v represents a valid BTC address.
//
// isvalid:rule
//	{ "name": "btc", "err": { "text": "must be a valid BTC address" } }
func BTC(v string) bool {
	return rxBTC.MatchString(v)
}

var rxBase32 = regexp.MustCompile(`^[A-Z2-7]+=*$`)

// Base32 reports whether or not v is a valid base32 string.
//
// isvalid:rule
//	{ "name": "base32", "err": { "text": "must be a valid base32 string" } }
func Base32(v string) bool {
	return (len(v)%8 == 0) && rxBase32.MatchString(v)
}

var rxBase58 = regexp.MustCompile(`^[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]*$`)

// Base58 reports whether or not v is a valid base58 string.
//
// isvalid:rule
//	{ "name": "base58", "err": { "text": "must be a valid base58 string" } }
func Base58(v string) bool {
	return rxBase58.MatchString(v)
}

// Base64 reports whether or not v is a valid base64 string. NOTE: The standard
// "encoding/base64" package is used for validation. With urlsafe=false StdEncoding
// is used and with urlsafe=true RawURLEncoding is used.
//
// isvalid:rule
//	{
//		"name": "base64",
//		"opts": [[
//			{ "key": null, "value": "false"	},
//			{ "key": "url", "value": "true" }
//		]],
//		"err": { "text": "must be a valid base64 string" }
// 	}
func Base64(v string, urlsafe bool) bool {
	if urlsafe {
		if i := strings.IndexAny(v, "\r\n"); i > -1 {
			return false
		}
		_, err := base64.RawURLEncoding.DecodeString(v)
		return err == nil
	}
	_, err := base64.StdEncoding.DecodeString(v)
	return err == nil
}

var rxBinary = regexp.MustCompile(`^(?:0[bB])?[0-1]+$`)

// Binary reports whether or not v represents a valid binary integer.
//
// isvalid:rule
//	{ "name": "binary", "err": { "text": "string content must match a binary number" } }
func Binary(v string) bool {
	return rxBinary.MatchString(v)
}

// Bool reports whether or not v represents a valid boolean. The following
// are considered valid boolean values: "true", "false", "TRUE", and "FALSE".
//
// isvalid:rule
//	{ "name": "bool", "err": { "text": "string content must match a boolean value" } }
func Bool(v string) bool {
	if len(v) == 4 && (v == "true" || v == "TRUE") {
		return true
	}
	if len(v) == 5 && (v == "false" || v == "FALSE") {
		return true
	}
	return false
}

// CIDR reports whether or not v is a valid Classless Inter-Domain Routing notation.
// NOTE: CIDR uses "net".ParseCIDR to determine the validity of v.
//
// isvalid:rule
//	{ "name": "cidr", "err": { "text": "must be a valid CIDR notation" } }
func CIDR(v string) bool {
	_, _, err := net.ParseCIDR(v)
	return err == nil
}

var rxCVV = regexp.MustCompile(`^[0-9]{3,4}$`)

// CVV reports whether or not v is a valid Card Verification Value.
//
// isvalid:rule
//	{ "name": "cvv", "err": { "text": "must be a valid CVV" } }
func CVV(v string) bool {
	return rxCVV.MatchString(v)
}

// Currency reports whether or not v is a valid Currency amount.
//
// isvalid:rule
//	{ "name": "currency", "err": {"text": "must be a valid currency amount"} }
func Currency(v string) bool {
	// TODO
	return false
}

var rxDataURIMediaType = regexp.MustCompile(`^(?i)[a-z]+\/[a-z0-9\-\+]+$`)
var rxDataURIAttribute = regexp.MustCompile(`^(?i)^[a-z\-]+=[a-z0-9\-]+$`)
var rxDataURIData = regexp.MustCompile(`^(?i)[a-z0-9!\$&'\(\)\*\+,;=\-\._~:@\/\?%\s]*$`)

// DataURI reports whether or not v is a valid data URI.
//
// isvalid:rule
//	{ "name": "datauri", "err": { "text": "must be a valid data URI" } }
func DataURI(v string) bool {
	vv := strings.Split(v, ",")
	if len(vv) < 2 {
		return false
	}

	attrs := strings.Split(strings.TrimSpace(vv[0]), ";")
	if attrs[0][:5] != "data:" {
		return false
	}

	mediaType := attrs[0][5:]
	if len(mediaType) > 0 && !rxDataURIMediaType.MatchString(mediaType) {
		return false
	}

	for i := 1; i < len(attrs); i++ {
		if i == len(attrs)-1 && strings.ToLower(attrs[i]) == "base64" {
			continue // ok
		}

		if !rxDataURIAttribute.MatchString(attrs[i]) {
			return false
		}
	}

	data := vv[1:]
	for i := 0; i < len(data); i++ {
		if !rxDataURIData.MatchString(data[i]) {
			return false
		}
	}
	return true
}

// Decimal reports whether or not v represents a valid decimal number.
//
// isvalid:rule
//	{ "name": "decimal", "err": { "text": "string content must match a decimal number" } }
func Decimal(v string) bool {
	// TODO
	return false
}

var rxDigits = regexp.MustCompile(`^[0-9]+$`)

// Digits reports whether or not v is a string of digits.
//
// isvalid:rule
//	{ "name": "digits", "err": {"text": "must contain only digits"} }
func Digits(v string) bool {
	return rxDigits.MatchString(v)
}

// EAN reports whether or not v is a valid European Article Number.
//
// isvalid:rule
//	{ "name": "ean", "err": { "text": "must be a valid EAN" } }
func EAN(v string) bool {
	length := len(v)
	if length != 8 && length != 13 {
		return false
	}
	if !rxDigits.MatchString(v) {
		return false
	}

	// the accumulate checksum
	checksum := 0
	for i, digit := range v[:length-1] {

		// the digit's weigth by position
		weight := 1
		if length == 8 && i%2 == 0 {
			weight = 3
		} else if length == 13 && i%2 != 0 {
			weight = 3
		}

		num, _ := strconv.Atoi(string(digit))
		checksum += num * weight
	}

	// the expected check digit
	want, _ := strconv.Atoi(string(v[length-1]))

	// the calculated check digit
	got := 0
	if remainder := (10 - (checksum % 10)); remainder < 10 {
		got = remainder
	}

	// do they match?
	return got == want
}

// EIN reports whether or not v is a valid Employer Identification Number.
//
// isvalid:rule
//	{ "name": "ein", "err": { "text": "must be a valid EIN" } }
func EIN(v string) bool {
	// TODO
	return false
}

var rxETH = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

// ETH reports whether or not v is a valid ethereum address.
//
// isvalid:rule
//	{ "name": "eth", "err": { "text": "must be a valid ethereum address" } }
func ETH(v string) bool {
	return rxETH.MatchString(v)
}

// Email reports whether or not v is a valid email address. NOTE: Email uses
// "net/mail".ParseAddress to determine the validity of v.
//
// isvalid:rule
//	{ "name": "email", "err": { "text": "must be a valid email address" } }
func Email(v string) bool {
	_, err := mail.ParseAddress(v)
	return err == nil
}

var rxTLD = regexp.MustCompile(`^(?i:[a-z\x{00a1}-\x{ffff}]{2,}|xn[a-z0-9-]{2,})$`)
var rxTLDIllegal = regexp.MustCompile(`[\s\x{2002}-\x{200B}\x{202F}\x{205F}\x{3000}\x{FEFF}\x{DB40}\x{DC20}\x{00A9}\x{FFFD}]`)
var rxFQDNPart = regexp.MustCompile(`^[a-zA-Z\x{00a1}-\x{ffff}0-9-]+$`)
var rxFQDNPartIllegal = regexp.MustCompile(`[\x{ff01}-\x{ff5e}]`)

// FQDN reports whether or not v is a valid Fully Qualified Domain Name. NOTE: FQDN
// TLD is required, numeric TLDs or trailing dots are disallowed, and underscores
// are forbidden.
//
// isvalid:rule
//	{
//		"name": "fqdn",
//		"err": { "text": "must be a valid FQDN" }
//	}
func FQDN(v string) bool {
	parts := strings.Split(v, ".")
	for _, part := range parts {
		if len(part) > 63 {
			return false
		}
	}

	// tld must be present, must match pattern, must not contain illegal chars, must not be all digits
	if len(parts) < 2 {
		return false
	}
	tld := parts[len(parts)-1]
	if !rxTLD.MatchString(tld) || rxTLDIllegal.MatchString(tld) || rxDigits.MatchString(tld) {
		return false
	}

	for _, part := range parts {
		if len(part) > 0 && (part[0] == '-' || part[len(part)-1] == '-') {
			return false
		}
		if !rxFQDNPart.MatchString(part) || rxFQDNPartIllegal.MatchString(part) {
			return false
		}
	}
	return true
}

var rxFloat = regexp.MustCompile(`^[+-]?(?:[0-9]*)?(?:\.[0-9]*)?(?:[eE][+-]?[0-9]+)?$`)

// Float reports whether or not v represents a valid float.
//
// isvalid:rule
//	{ "name": "float", "err": {"text": "string content must match a floating point number"} }
func Float(v string) bool {
	if v == "" || v == "." || v == "+" || v == "-" {
		return false
	}
	return rxFloat.MatchString(v)
}

var rxHSLComma = regexp.MustCompile(`^(?i)(?:hsl)a?\(\s*(?:(?:\+|\-)?(?:[0-9]+(?:\.[0-9]+)?(?:e(?:\+|\-)?[0-9]+)?|\.[0-9]+(?:e(?:\+|\-)?[0-9]+)?))(?:deg|grad|rad|turn|\s*)(?:\s*,\s*(?:\+|\-)?(?:[0-9]+(?:\.[0-9]+)?(?:e(?:\+|\-)?[0-9]+)?|\.[0-9]+(?:e(?:\+|\-)?[0-9]+)?)%){2}\s*(?:,\s*(?:(?:\+|\-)?(?:[0-9]+(?:\.[0-9]+)?(?:e(?:\+|\-)?[0-9]+)?|\.[0-9]+(?:e(?:\+|\-)?[0-9]+)?)%?)\s*)?\)$`)
var rxHSLSpace = regexp.MustCompile(`^(?i)(?:hsl)a?\(\s*(?:(?:\+|\-)?(?:[0-9]+(?:\.[0-9]+)?(?:e(?:\+|\-)?[0-9]+)?|\.[0-9]+(?:e(?:\+|\-)?[0-9]+)?))(?:deg|grad|rad|turn|\s)(?:\s*(?:\+|\-)?(?:[0-9]+(?:\.[0-9]+)?(?:e(?:\+|\-)?[0-9]+)?|\.[0-9]+(?:e(?:\+|\-)?[0-9]+)?)%){2}\s*(?:\/\s*(?:(?:\+|\-)?(?:[0-9]+(?:\.[0-9]+)?(?:e(?:\+|\-)?[0-9]+)?|\.[0-9]+(?:e(?:\+|\-)?[0-9]+)?)%?)\s*)?\)$`)

// HSL reports whether or not v represents an HSL color value.
//
// isvalid:rule
//	{ "name": "hsl", "err": {"text": "must be a valid HSL color"} }
func HSL(v string) bool {
	return rxHSLComma.MatchString(v) || rxHSLSpace.MatchString(v)
}

var hashAlgoLengths = map[string]int{
	"md5":       32,
	"md4":       32,
	"sha1":      40,
	"sha256":    64,
	"sha384":    96,
	"sha512":    128,
	"ripemd128": 32,
	"ripemd160": 40,
	"tiger128":  32,
	"tiger160":  40,
	"tiger192":  48,
	"crc32":     8,
	"crc32b":    8,
}

var rxHash = regexp.MustCompile(`^[0-9A-Fa-f]+$`)

// Hash reports whether or not v is a hash of the specified algorithm.
//
// isvalid:rule
//	{ "name": "hash", "err": {"text": "must be a valid hash"} }
func Hash(v string, algo string) bool {
	if length := hashAlgoLengths[algo]; length > 0 && length == len(v) {
		return rxHash.MatchString(v)
	}
	return false
}

var rxHex = regexp.MustCompile(`^(?:0[xXhH])?[0-9A-Fa-f]+$`)

// Hex reports whether or not v is a valid hexadecimal string.
//
// isvalid:rule
//	{ "name": "hex", "err": { "text": "must be a valid hexadecimal string" } }
func Hex(v string) bool {
	return rxHex.MatchString(v)
}

var rxHexColor = regexp.MustCompile(`^#?(?i:[0-9A-F]{3}|[0-9A-F]{4}|[0-9A-F]{6}|[0-9A-F]{8})$`)

// HexColor reports whether or not v is a valid hexadecimal color code.
//
// isvalid:rule
//	{ "name": "hexcolor", "err": { "text": "must represent a valid hexadecimal color code" } }
func HexColor(v string) bool {
	return rxHexColor.MatchString(v)
}

// IBAN reports whether or not v is an International Bank Account Number.
//
// isvalid:rule
//	{ "name": "iban", "err": { "text": "must be a valid IBAN" } }
func IBAN(v string) bool {
	v = rmchar(v, func(r rune) bool { return r == ' ' || r == '-' })
	if len(v) < 2 {
		return false
	}

	v = strings.ToUpper(v)
	if rx, ok := locale.IBANRegexpTable[v[:2]]; !ok || !rx.MatchString(v) {
		return false
	}

	// rearrange by moving the four initial characters to the end of the string
	v = v[4:] + v[:4]

	// convert to decimal int by replacing each letter in the string with two digits
	var D string
	for _, r := range v {
		if r >= 'A' && 'Z' >= r {
			D += strconv.Itoa(int(r - 55))
		} else {
			D += string(r)
		}
	}

	// NOTE: The D here, based on the above steps, is known to contain only
	// digits which is why the checking of errors from strconv.Atoi is omitted.
	//
	// The modulo algorithm for checking the IBAN is taken from:
	// https://en.wikipedia.org/wiki/International_Bank_Account_Number#Modulo_operation_on_IBAN
	//
	// 1. Starting from the leftmost digit of D, construct a number using
	//    the first 9 digits and call it N.
	// 2. Calculate N mod 97.
	// 3. Construct a new 9-digit N by concatenating above result (step 2)
	//    with the next 7 digits of D. If there are fewer than 7 digits remaining
	//    in D but at least one, then construct a new N, which will have less
	//    than 9 digits, from the above result (step 2) followed by the remaining
	//    digits of D.
	// 4. Repeat steps 2–3 until all the digits of D have been processed.
	//
	d, D := D[:9], D[9:]
	for len(D) > 0 {
		N, _ := strconv.Atoi(d)
		mod := strconv.Itoa(N % 97)

		if len(D) >= 7 {
			d, D = (mod + D[:7]), D[7:]
		} else {
			d, D = (mod + D), ""
		}
	}

	N, _ := strconv.Atoi(d)
	return (N % 97) == 1
}

// IC reports whether or not v is an Identity Card number.
//
// isvalid:rule
//	{ "name": "ic", "err": { "text": "must be a valid identity card number" } }
func IC(v string) bool {
	// TODO
	return false
}

// IMEI reports whether or not v is an IMEI number.
//
// isvalid:rule
//	{ "name": "imei", "err": { "text": "must be a valid IMEI number" } }
func IMEI(v string) bool {
	// TODO
	return false
}

// IP reports whether or not v is a valid IP address. The ver argument specifies
// the IP's expected version. The ver argument can be one of the following three
// values:
//
//	0 accepts both IPv4 and IPv6
//	4 accepts IPv4 only
//	6 accepts IPv6 only
//
// isvalid:rule
//	{
//		"name": "ip",
//		"opts": [[
//			{ "key": null, "value": "0" },
//			{ "key": "v4", "value": "4" },
//			{ "key": "v6", "value": "6" }
//		]],
//		"err": { "text": "must be a valid IP" }
//	}
func IP(v string, ver int) bool {
	// TODO
	return false
}

// IPRange reports whether or not v is a valid IP range (IPv4 only).
//
// isvalid:rule
//	{ "name": "iprange", "err": { "text": "must be a valid IP range" } }
func IPRange(v string) bool {
	// TODO
	return false
}

// ISBN reports whether or not v is a valid International Standard Book Number.
// The ver argument specifies the ISBN's expected version. The ver argument can
// be one of the following three values:
//
//	0 accepts both 10 and 13
//	10 accepts version 10 only
//	13 accepts version 13 only
//
// isvalid:rule
//	{
//		"name": "isbn",
//		"opts": [[ { "key": null, "value": "0" } ]],
//		"err": { "text": "must be a valid ISBN" }
//	}
func ISBN(v string, ver int) bool {
	// TODO
	return false
}

// ISIN reports whether or not v is a valid International Securities Identification Number.
//
// isvalid:rule
//	{ "name": "isin", "err": { "text": "must be a valid ISIN" } }
func ISIN(v string) bool {
	// TODO
	return false
}

// ISO reports whether or not v is a valid representation of the specified ISO standard.
//
// isvalid:rule
//	{ "name": "iso", "err": { "text": "must be a valid ISO", "with_opts": true } }
func ISO(v string, num int) bool {
	// TODO
	return false
}

// ISO31661A reports whether or not v is a valid country code as defined by the
// ISO 3166-1 Alpha standard. The num argument specifies the which of the two
// alpha sets of the standard should be tested. The num argument can be one of
// the following three values:
//
//	0 tests against both Alpha-2 and Alpha-3
//	2 tests against Alpha-2 only
//	3 tests against Alpha-3 only
//
// isvalid:rule
//	{
//		"name": "iso31661a",
//		"opts": [[ { "key": null, "value": "0" } ]],
//		"err": { "text": "must be a valid ISO 3166-1 Alpha value" }
//	}
func ISO31661A(v string, num int) bool {
	// TODO
	return false
}

// ISRC reports whether or not v is a valid International Standard Recording Code.
//
// isvalid:rule
//	{ "name": "isrc", "err": { "text": "must be a valid ISRC" } }
func ISRC(v string) bool {
	// TODO
	return false
}

// ISSN reports whether or not v is a valid International Standard Serial Number.
//
// isvalid:rule
//	{ "name": "issn", "err": { "text": "must be a valid ISSN" } }
func ISSN(v string) bool {
	// TODO
	return false
}

// In reports whether or not v is in the list.
//
// isvalid:rule
//	{ "name": "in", "err": { "text": "must be in the list" } }
func In(v interface{}, list ...interface{}) bool {
	// TODO
	return false
}

var rxInt = regexp.MustCompile(`^[+-]?[0-9]+$`)

// Int reports whether or not v represents a valid integer.
//
// isvalid:rule
//	{ "name": "int", "err": { "text": "string content must match an integer" } }
func Int(v string) bool {
	return rxInt.MatchString(v)
}

// JSON reports whether or not v is valid JSON.
//
// isvalid:rule
//	{ "name": "json", "err": { "text": "must be a valid JSON" } }
func JSON(v string) bool {
	// TODO
	return false
}

// JWT reports whether or not v is a valid JSON Web Token.
//
// isvalid:rule
//	{ "name": "jwt", "err": { "text": "must be a valid JWT" } }
func JWT(v string) bool {
	// TODO
	return false
}

// LatLong reports whether or not v is a valid latitude-longitude coordinate string.
//
// isvalid:rule
//	{ "name": "latlong", "err": {"text": "must be a valid latitude-longitude coordinate"} }
func LatLong(v string) bool {
	// TODO
	return false
}

// Locale reports whether or not v is a valid locale.
//
// isvalid:rule
//	{ "name": "locale", "err": {"text": "must be a valid locale"} }
func Locale(v string) bool {
	// TODO
	return false
}

// LowerCase reports whether or not v is an all lower-case string.
//
// isvalid:rule
//	{ "name": "lower", "err": {"text": "must contain only lower-case characters"} }
func LowerCase(v string) bool {
	return v == strings.ToLower(v)
}

var rxMAC6 = regexp.MustCompile(`^[0-9a-fA-F]{12}$`)
var rxMAC6Colon = regexp.MustCompile(`^(?:[0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$`)
var rxMAC6Hyphen = regexp.MustCompile(`^(?:[0-9a-fA-F]{2}-){5}[0-9a-fA-F]{2}$`)

var rxMAC8 = regexp.MustCompile(`^[0-9a-fA-F]{16}$`)
var rxMAC8Colon = regexp.MustCompile(`^(?:[0-9a-fA-F]{2}:){7}[0-9a-fA-F]{2}$`)
var rxMAC8Hyphen = regexp.MustCompile(`^(?:[0-9a-fA-F]{2}-){7}[0-9a-fA-F]{2}$`)

// MAC reports whether or not v is a valid MAC address. The space argument specifies
// the identifier's expected address space (in bytes). The space argument can be one
// of the following three values:
//
//	0 accepts both EUI-48 and EUI-64
//	6 accepts EUI-48 format only
//	8 accepts EUI-64 format only
//
// The allowed formatting of the identifiers is as follows:
//
//	// EUI-48 format
//	"08:00:2b:01:02:03"
//	"08-00-2b-01-02-03"
//	"08002b010203"
//
//	// EUI-64 format
//	"08:00:2b:01:02:03:04:05"
//	"08-00-2b-01-02-03-04-05"
//	"08002b0102030405"
//
// isvalid:rule
//	{
//		"name": "mac",
//		"opts": [[ { "key": null, "value": "0" } ]],
//		"err": { "text": "must be a valid MAC" }
//	}
func MAC(v string, space int) bool {
	if space == 0 {
		return rxMAC6.MatchString(v) ||
			rxMAC6Colon.MatchString(v) ||
			rxMAC6Hyphen.MatchString(v) ||
			rxMAC8.MatchString(v) ||
			rxMAC8Colon.MatchString(v) ||
			rxMAC8Hyphen.MatchString(v)
	} else if space == 6 {
		return rxMAC6.MatchString(v) ||
			rxMAC6Colon.MatchString(v) ||
			rxMAC6Hyphen.MatchString(v)
	} else if space == 8 {
		return rxMAC8.MatchString(v) ||
			rxMAC8Colon.MatchString(v) ||
			rxMAC8Hyphen.MatchString(v)
	}
	return false
}

// MD5 reports whether or not v is a valid MD5 hash.
//
// isvalid:rule
//	{ "name": "md5", "err": {"text": "must be a valid MD5 hash"} }
func MD5(v string) bool {
	// TODO
	return false
}

// MagnetURI reports whether or not v is a valid magned URI.
//
// isvalid:rule
//	{ "name": "magneturi", "err": {"text": "must be a valid magnet URI"} }
func MagnetURI(v string) bool {
	// TODO
	return false
}

// Match reports whether or not the v contains any match of the regular expression re.
// NOTE: Match will panic if re has not been registered upfront with RegisterRegexp.
//
// isvalid:rule
//	{ "name": "re", "err": {"text": "must match the regular expression", "with_opts": true} }
func Match(v string, re string) bool {
	return regexpCache.m[re].MatchString(v)
}

// MediaType reports whether or not v is a valid Media type (or MIME type).
//
// isvalid:rule
//	{ "name": "mediatype", "err": {"text": "must be a valid media type"} }
func MediaType(v string) bool {
	// TODO
	return false
}

// MongoId reports whether or not v is a valid hex-encoded representation of a MongoDB ObjectId.
//
// isvalid:rule
//	{ "name": "mongoid", "err": {"text": "must be a valid Mongo Object Id"} }
func MongoId(v string) bool {
	// TODO
	return false
}

var rxNumeric = regexp.MustCompile(`^[+-]?[0-9]*\.?[0-9]+$`)

// Numeric reports whether or not v is a valid numeric string.
//
// isvalid:rule
//	{ "name": "numeric", "err": {"text": "string content must match a numeric value"} }
func Numeric(v string) bool {
	return rxNumeric.MatchString(v)
}

var rxOctal = regexp.MustCompile(`^(?:0[oO])?[0-7]+$`)

// Octal reports whether or not v represents a valid octal integer.
//
// isvalid:rule
//	{ "name": "octal", "err": {"text": "string content must match an octal number"} }
func Octal(v string) bool {
	return rxOctal.MatchString(v)
}

var rxPAN = regexp.MustCompile(`^(?:4[0-9]{12}(?:[0-9]{3,6})?|5[1-5][0-9]{14}|(?:222[1-9]|22[3-9][0-9]|2[3-6][0-9]{2}|27[01][0-9]|2720)[0-9]{12}|6(?:011|5[0-9][0-9])[0-9]{12,15}|3[47][0-9]{13}|3(?:0[0-5]|[68][0-9])[0-9]{11}|(?:2131|1800|35\d{3})\d{11}|6[27][0-9]{14})$`)

// PAN reports whether or not v is a valid Primary Account Number or Credit Card number.
//
// isvalid:rule
//	{ "name": "pan", "err": { "text": "must be a valid PAN" } }
func PAN(v string) bool {
	v = rmchar(v, func(r rune) bool { return r == ' ' || r == '-' })
	if !rxPAN.MatchString(v) {
		return false
	}

	// luhn check
	var sum int
	var double bool
	for i := len(v) - 1; i >= 0; i-- {
		num, _ := strconv.Atoi(string(v[i]))

		if double {
			num *= 2
			if num > 9 {
				num = (num % 10) + 1
			}
		}
		double = !double

		sum += num
	}
	return sum%10 == 0
}

// PassportNumber reports whether or not v is a valid passport number.
//
// isvalid:rule
//	{ "name": "passport", "err": { "text": "must be a valid passport number" } }
func PassportNumber(v string) bool {
	// TODO
	return false
}

// Phone reports whether or not v is a valid phone number.
//
// isvalid:rule
//	{ "name": "phone", "err": { "text": "must be a valid phone number" } }
func Phone(v string, cc ...string) bool {
	// TODO
	return false
}

// Port reports whether or not v is a valid port number.
//
// isvalid:rule
//	{ "name": "port", "err": { "text": "must be a valid port number" } }
func Port(v string) bool {
	// TODO
	return false
}

// RFC reports whether or not v is a valid representation of the specified RFC standard.
//
// isvalid:rule
//	{ "name": "rfc", "err": { "text": "must be a valid RFC", "with_opts": true } }
func RFC(v string, num int) bool {
	// TODO
	return false
}

// RGB reports whether or not v is a valid RGB color.
//
// isvalid:rule
//	{ "name": "rgb", "err": { "text": "must be a valid RGB color" } }
func RGB(v string) bool {
	// TODO
	return false
}

// SSN reports whether or not v is a valid Social Security Number.
//
// isvalid:rule
//	{ "name": "ssn", "err": { "text": "must be a valid SSN" } }
func SSN(v string) bool {
	// TODO
	return false
}

// SemVer reports whether or not v is a valid Semantic Versioning number.
//
// isvalid:rule
//	{ "name": "semver", "err": { "text": "must be a valid semver number" } }
func SemVer(v string) bool {
	// TODO
	return false
}

// Slug reports whether or not v is a valid slug.
//
// isvalid:rule
//	{ "name": "slug", "err": { "text": "must be a valid slug" } }
func Slug(v string) bool {
	// TODO
	return false
}

// StrongPassword reports whether or not v is a strong password.
//
// isvalid:rule
//	{ "name": "strongpass", "err": { "text": "must be a strong password" } }
func StrongPassword(v string) bool {
	// TODO
	return false
}

// URI reports whether or not v is a valid Uniform Resource Identifier.
//
// isvalid:rule
//	{ "name": "uri", "err": { "text": "must be a valid URI" } }
func URI(v string) bool {
	// TODO
	return false
}

// URL reports whether or not v is a valid Uniform Resource Locator.
//
// isvalid:rule
//	{ "name": "url", "err": {"text": "must be a valid URL"} }
func URL(v string) bool {
	// TODO
	return false
}

// UUID reports whether or not v is a valid Universally Unique IDentifier.
//
// isvalid:rule
//	{ "name": "uuid", "opt_min": 0, "opt_max": 5, "err": {"text": "must be a valid UUID"} }
func UUID(v string, ver ...int) bool {
	// TODO
	return false
}

var rxUint = regexp.MustCompile(`^\+?[0-9]+$`)

// Uint reports whether or not v represents a valid unsigned integer.
//
// isvalid:rule
//	{ "name": "uint", "err": {"text": "string content must match an unsigned integer"} }
func Uint(v string) bool {
	return rxUint.MatchString(v)
}

// UpperCase reports whether or not v is an all upper-case string.
//
// isvalid:rule
//	{ "name": "upper", "err": { "text": "must contain only upper-case characters" } }
func UpperCase(v string) bool {
	return v == strings.ToUpper(v)
}

// VAT reports whether or not v is a valid Value Added Tax number.
//
// isvalid:rule
//	{ "name": "vat", "err": {"text": "must be a valid VAT number"} }
func VAT(v string) bool {
	// TODO
	return false
}

// Zip reports whether or not v is a valid zip code.
//
// isvalid:rule
//	{ "name": "zip", "err": {"text": "must be a valid zip code"} }
func Zip(v string, cc ...string) bool {
	// TODO
	return false
}

// remove characters from string
func rmchar(v string, f func(r rune) bool) string {
	return strings.Map(func(r rune) rune {
		if f(r) {
			return -1
		}
		return r
	}, v)

}
