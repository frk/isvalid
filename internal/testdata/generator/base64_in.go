package testdata

type Base64Validator struct {
	F1 string   `is:"base64"`
	F2 **string `is:"base64:true"`
	F3 **string `is:"required,base64"`
}
