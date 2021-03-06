package analysis

import (
	"reflect"
	"regexp"

	"github.com/frk/isvalid"
)

// TagNode is a binary tree representation of a parsed "rule" tag.
type TagNode struct {
	// The list of rules contained in the node.
	Rules []*Rule
	// Key and Elem are the child nodes of the parent node.
	Key, Elem *TagNode
}

// HasRuleRequired reports whether or not the TagNode contains the rule "required".
func (tn *TagNode) HasRuleRequired() bool {
	if tn != nil {
		for _, r := range tn.Rules {
			if r.Name == "required" {
				return true
			}
		}
	}
	return false
}

// HasRuleRequired reports whether or not the TagNode contains the rule "notnil".
func (tn *TagNode) HasRuleNotnil() bool {
	if tn != nil {
		for _, r := range tn.Rules {
			if r.Name == "notnil" {
				return true
			}
		}
	}
	return false
}

// ContainsRules reports whether or not the TagNode tn, or any of
// the TagNodes in the key-elem hierarchy of tn, contain validation rules.
func (tn *TagNode) ContainsRules() bool {
	if tn != nil {
		if len(tn.Rules) > 0 {
			return true
		}
		if tn.Key.ContainsRules() {
			return true
		}
		if tn.Elem.ContainsRules() {
			return true
		}
	}
	return false
}

var rxBool = regexp.MustCompile(`^(?:false|true)$`)

// parseRuleTag parses the given tag and returns a node that represents the
// tag as a binary tree. Following is an *incomplete* attempt to describe the
// expected format of the "rule" tag in EBNF:
//
//      node      = rule | [ "[" [ node ] "]" ] [ ( node | rule "," node ) ] .
//      rule      = rule_name [ { ":" rule_opt } ] { "," rule } .
//      rule_name = identifier .
//      rule_opt  = | boolean_lit | integer_lit | float_lit | string_lit | quoted_string_lit | field_reference | context_property .
//
//      boolean_lit       = "true" | "false" .
//      integer_lit       = "0" | [ "-" ] "1"…"9" { "0"…"9" } .
//      float_lit         = [ "-" ] ( "0" | "1"…"9" { "0"…"9" } ) "." "0"…"9" { "0"…"9" } .
//      string_lit        = .
//      quoted_string_lit = `"` `"` .
//
//      field_reference     = "&" field_key .
//      field_key           = identifier { field_key_separator identifier } .
//      field_key_separator = "." | (* optionally specified by the user *)
//
//      context_property  = "@" identifier .
//
//      identifier        = letter { letter } .
//      letter            = "A"…"Z" | "a"…"z" | "_" .
//
func parseRuleTag(tag string) (*TagNode, error) {
	val, ok := reflect.StructTag(tag).Lookup("is")
	if !ok || val == "-" || len(val) == 0 {
		return &TagNode{}, nil
	}

	// parser is invoked recursively to parse tags enclosed in square brackets.
	var parser func(tag string) (*TagNode, error)
	parser = func(tag string) (*TagNode, error) {
		tn := &TagNode{}
		for tag != "" {
			// skip leading space
			i := 0
			for i < len(tag) && tag[i] == ' ' {
				i++
			}
			tag = tag[i:]
			if tag == "" {
				break
			}

			// parse bracketed rules
			if tag[0] == '[' {

				// scan up to the *matching* closing bracket
				i, n := 1, 0
				for i < len(tag) && (tag[i] != ']' || n > 0) {
					// adjust nesting level
					if tag[i] == '[' {
						n++
					} else if tag[i] == ']' {
						n--
					}
					i++

					// scan quoted string, ignoring brackets inside quotes
					if tag[i-1] == '"' {
						for i < len(tag) && tag[i] != '"' {
							if tag[i] == '\\' {
								i++
							}
							i++
						}

						// keep the closing double quote, or
						// else the subsequent parser calls
						// will be confused without it
						if i < len(tag) {
							i++
						}
					}
				}

				// recursively invoke parser for key
				if ktag := tag[1:i]; len(ktag) > 0 {
					key, err := parser(ktag)
					if err != nil {
						return nil, err
					}
					tn.Key = key
				}
				// recursively invoke parser for elem
				if etag := tag[i:]; len(etag) > 1 {
					etag = etag[1:] // drop the leading ']'
					elem, err := parser(etag)
					if err != nil {
						return nil, err
					}
					tn.Elem = elem
				}

				// done; exit
				return tn, nil
			}

			// scan to the end of a rule's name
			i = 0
			for i < len(tag) && tag[i] != ',' && tag[i] != ':' {
				i++
			}

			// empty name's no good; next
			if tag[:i] == "" {
				tag = tag[1:]
				continue
			}

			r := &Rule{Name: tag[:i]}
			tn.Rules = append(tn.Rules, r)

			// this rule's done; next or exit
			if tag = tag[i:]; tag == "" {
				break
			} else if tag[0] == ',' {
				tag = tag[1:]
				continue
			}

			// scan the rule's options
			for tag != "" {
				tag = tag[1:] // drop the leading ':'

				// quoted option value; scan to the end quote
				if len(tag) > 0 && tag[0] == '"' {
					i := 1
					for i < len(tag) && tag[i] != '"' {
						if tag[i] == '\\' {
							i++
						}
						i++
					}

					opt := &RuleOption{}
					opt.Value = tag[1:i]
					opt.Type = OptionTypeString
					r.Options = append(r.Options, opt)

					tag = tag[i:]

					// drop the closing quote
					if len(tag) > 0 && tag[0] == '"' {
						tag = tag[1:]
					}

					// next option?
					if len(tag) > 0 && tag[0] == ':' {
						continue
					}

					// drop rule separator
					if len(tag) > 0 && tag[0] == ',' {
						tag = tag[1:]
					}

					// this rule's done; exit
					break
				}

				// scan to the end of a rule's option
				i := 0
				for i < len(tag) && tag[i] != ':' && tag[i] != ',' {
					i++
				}

				optstr := tag[:i]
				if len(optstr) > 0 && optstr[0] == '@' {
					r.Context = optstr[1:]
				} else {
					opt := parseRuleTagOption(optstr)
					r.Options = append(r.Options, opt)
				}

				tag = tag[i:]
				if tag == "" {
					break
				} else if tag[0] == ',' {
					tag = tag[1:]
					break
				}
			}
		}
		return tn, nil
	}

	return parser(val)
}

// parseRuleTagOption parses the given as a RuleOption and returns the result.
func parseRuleTagOption(val string) (opt *RuleOption) {
	opt = &RuleOption{}
	if len(val) > 0 {
		if val[0] == '&' {
			opt.Value = val[1:]
			opt.Type = OptionTypeField
		} else {
			opt.Value = val
			switch {
			case isvalid.Int(val):
				opt.Type = OptionTypeInt
			case isvalid.Float(val):
				opt.Type = OptionTypeFloat
			case rxBool.MatchString(val):
				opt.Type = OptionTypeBool
			case val != `nil`:
				opt.Type = OptionTypeString
			}
		}
	}
	return opt
}
