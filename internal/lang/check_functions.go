package lang

import "fmt"

// checkFunctionCall resolves callable bindings before ordinary words. A
// same-named non-callable value does not shadow an existing word call.
func (c *checker) checkFunctionCall(x *expression, env *scope) (valueType, bool, error) {
	var params []valueType
	var result valueType
	minimum := 0
	callable := false
	if binding := env.find(x.text); binding != nil {
		if p, r, ok := functionTypeParts(binding.typ); ok {
			x.binding, params, result = binding, p, r
			callable = true
			minimum = len(params)
		}
	}
	if !callable {
		w, private := c.resolveWord(x.text)
		if w == nil || private != nil {
			return "", false, nil
		}
		x.word, result = w, w.result
		for _, param := range w.params {
			params = append(params, param.typ)
		}
		minimum = len(params)
		for minimum > 0 && w.params[minimum-1].defaultExpr != nil {
			minimum--
		}
	}
	holes := []int{}
	for i, arg := range x.args {
		if arg.kind == "hole" {
			holes = append(holes, i)
		}
	}
	if x.binding == nil && len(holes) == 0 {
		return "", false, nil
	}
	if len(x.argNames) != 0 {
		return "", true, errorAt(x.pos, CodeArgumentType, "named arguments are only supported for record constructors")
	}
	if len(x.args) < minimum || len(x.args) > len(params) {
		return "", true, withSpan(errorAt(x.pos, CodeArgumentCount, "%s requires %d to %d arguments, found %d", x.text, minimum, len(params), len(x.args)), x)
	}
	for i, arg := range x.args {
		if arg.kind == "hole" {
			arg.typ = params[i]
			continue
		}
		typ, err := c.expressionExpected(arg, env, params[i])
		if err != nil {
			return "", true, err
		}
		if typ != params[i] {
			return "", true, valueError(x.pos, arg, CodeArgumentType, fmt.Sprintf("argument %d of %s requires type %s, found %s", i+1, x.text, params[i], typ), tryHint(typ), arg)
		}
		if typ == errorType && arg.binding != nil && arg.binding.errorFamily != "" && arg.binding.errorFamily != "Error" {
			return "", true, errorAt(x.pos, CodeArgumentType, "convert %s explicitly with error.wrap before passing it to a user word", arg.binding.errorFamily)
		}
	}
	if len(holes) > 0 {
		x.partial, x.holes = true, holes
		missing := make([]valueType, len(holes))
		for i, pos := range holes {
			missing[i] = params[pos]
		}
		x.typ = functionType(missing, result)
	} else {
		x.typ = result
	}
	return x.typ, true, nil
}
