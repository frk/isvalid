package testdata

type ContainsValidator struct {
	F1 string   `is:"contains:foo"`
	F2 *string  `is:"contains:bar"`
	F3 **string `is:"required,contains:foo:bar:baz"`
}
