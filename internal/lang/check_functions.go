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
		if w == nil {
			return "", false, nil
		}
		if private != nil {
			return "", true, withHint(withSpan(errorAt(x.pos, CodePrivateUse, "%q is private to module %q", w.name, private.name), x), fmt.Sprintf("remove the leading underscore in %s to export it", private.path))
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

// checkBuiltinPartial specializes a builtin once from its captures and any
// contextual function type. Unresolved overloads require an annotation.
func (c *checker) checkBuiltinPartial(x *expression, env *scope) (valueType, bool, error) {
	var holes []int
	for i, arg := range x.args {
		if arg.kind == "hole" {
			holes = append(holes, i)
		}
	}
	if len(holes) == 0 {
		return "", false, nil
	}
	if choice, module := c.privateVariant(x.text); choice != nil {
		return "", true, errorAt(x.pos, CodePrivateUse, "%q is private to module %q", choice.name, module.name)
	}
	if record, module := c.privateRecord(x.text); record != nil {
		return "", true, withHint(errorAt(x.pos, CodePrivateUse, "%q is private to module %q", record.name, module.name), fmt.Sprintf("remove the leading underscore in %s to export it", module.path))
	}
	if choice, _ := c.resolveVariant(x.text); choice != nil {
		return "", true, errorAt(x.pos, CodeArgumentType, "choice constructors cannot be partially applied")
	}
	if c.types.visible[x.text] != nil {
		return "", true, errorAt(x.pos, CodeArgumentType, "record constructors cannot be partially applied")
	}
	if !HostWord(x.text) {
		return "", true, withSpan(withHint(errorAt(x.pos, CodeUnknownWord, "unknown word %q", x.text), suggestWordHint(x.text, c.wordNames())), x)
	}
	switch x.text {
	case "range", "file.lines", "error.set-reporter", "ok", "err", "console.exit", "console.fail":
		return "", true, errorAt(x.pos, CodeArgumentType, "%s cannot be partially applied", x.text)
	}
	terms := inferScope{}
	for scope := env; scope != nil; scope = scope.parent {
		for name, binding := range scope.bindings {
			if terms[name] == nil {
				terms[name] = termFromType(binding.typ)
			}
		}
	}
	solver := &returnInference{checker: c}
	args := make([]*typeConstraint, len(x.args))
	for i, arg := range x.args {
		args[i] = solver.expr(arg, terms, false)
	}
	result := solver.call(x, args, false)
	expectedType := c.expected
	if expectedType == "" {
		expectedType = x.inferredPartial
	}
	if expected, expectedResult, ok := functionTypeParts(expectedType); ok {
		if len(expected) != len(holes) {
			return "", true, errorAt(x.pos, CodeArgumentCount, "partial requires %d hole types, found %d", len(expected), len(holes))
		}
		for i, position := range holes {
			solver.equal(args[position], termFromType(expected[i]), x.pos)
		}
		solver.equal(result, termFromType(expectedResult), x.pos)
	}
	for len(solver.pending) > 0 && solver.err == nil {
		remaining := solver.pending[:0]
		for _, rule := range solver.pending {
			if !rule() {
				remaining = append(remaining, rule)
			}
		}
		if len(remaining) == len(solver.pending) {
			break
		}
		solver.pending = remaining
	}
	if solver.err != nil {
		return "", true, solver.err
	}
	if len(solver.pending) != 0 {
		return "", true, withHint(errorAt(x.pos, CodeArgumentType, "cannot infer partial argument types"), "add a function type annotation")
	}
	for _, rule := range solver.allowed {
		typ, ok := constraintType(rule.value)
		if !ok {
			return "", true, withHint(errorAt(x.pos, CodeArgumentType, "cannot infer an overloaded partial"), "add a function type annotation")
		}
		found := false
		for _, name := range rule.names {
			if string(typ) == name {
				found = true
				break
			}
		}
		if !found {
			return "", true, errorAt(x.pos, CodeArgumentType, "type %s is not supported", typ)
		}
	}
	argTypes := make([]valueType, len(args))
	for i, term := range args {
		typ, ok := constraintType(term)
		if !ok {
			return "", true, withHint(errorAt(x.pos, CodeArgumentType, "cannot infer partial argument types"), "add a function type annotation")
		}
		argTypes[i] = typ
		if x.args[i].kind == "hole" {
			x.args[i].typ = typ
			continue
		}
		actual, err := c.expressionExpected(x.args[i], env, typ)
		if err != nil {
			return "", true, err
		}
		if actual != typ {
			return "", true, errorAt(x.args[i].pos, CodeArgumentType, "partial capture requires %s, found %s", typ, actual)
		}
		if actual == errorType && x.args[i].binding != nil && x.args[i].binding.errorFamily != "" && x.args[i].binding.errorFamily != "Error" {
			return "", true, errorAt(x.pos, CodeArgumentType, "convert %s explicitly with error.wrap before capturing it", x.args[i].binding.errorFamily)
		}
	}
	x.checkedCollection = false
	resultType, err := c.call(x, argTypes)
	if err != nil {
		return "", true, err
	}
	if resultType == neverType {
		return "", true, errorAt(x.pos, CodeArgumentType, "non-returning words cannot be partially applied")
	}
	missing := make([]valueType, len(holes))
	for i, pos := range holes {
		missing[i] = argTypes[pos]
	}
	x.partial, x.holes, x.typ = true, holes, functionType(missing, resultType)
	return x.typ, true, nil
}
