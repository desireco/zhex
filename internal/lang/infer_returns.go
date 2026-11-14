package lang

import (
	"fmt"
	"strings"
)

// returnInference collects constraints on fresh terms, leaving the checked AST
// untouched until all omitted signatures have concrete solutions.
type returnInference struct {
	checker      *checker
	results      map[*word]*typeConstraint
	params       map[*word][]*typeConstraint
	entry        []*statement
	entryScope   *scope
	views        map[*word]*checker
	entryView    *checker
	effects      map[*word]map[string]bool
	families     map[*word]*typeConstraint
	fallible     map[*word]bool
	partialTerms map[*expression]*typeConstraint
	current      *word
	err          error
	pending      []func() bool
	allowed      []allowedTypes
}
type allowedTypes struct {
	value *typeConstraint
	names []string
	pos   position
}
type inferScope map[string]*typeConstraint

func freshType() *typeConstraint { return &typeConstraint{} }
func namedType(name string, args ...*typeConstraint) *typeConstraint {
	return &typeConstraint{name: name, args: args}
}
func copyInferScope(env inferScope) inferScope {
	out := make(inferScope, len(env))
	for k, v := range env {
		out[k] = v
	}
	return out
}
func termFromType(typ valueType) *typeConstraint {
	if base, args, ok := typeApplication(typ); ok {
		values := make([]*typeConstraint, len(args))
		for i, arg := range args {
			values[i] = termFromType(arg)
		}
		return namedType(base, values...)
	}
	return namedType(string(typ))
}
func templateTerm(typ valueType, name string, param *typeConstraint) *typeConstraint {
	if string(typ) == name {
		return param
	}
	if base, args, ok := typeApplication(typ); ok {
		values := make([]*typeConstraint, len(args))
		for i, arg := range args {
			values[i] = templateTerm(arg, name, param)
		}
		return namedType(base, values...)
	}
	return termFromType(typ)
}
func (s *returnInference) equal(a, b *typeConstraint, pos position) {
	if s.err == nil && !unifyTypes(a, b) {
		s.err = withHint(errorAt(pos, CodeWordResult, "incompatible inferred types"), "add an explicit return annotation or correct the conflicting values")
	}
}
func (s *returnInference) constrain(t *typeConstraint, pos position, names ...string) {
	s.allowed = append(s.allowed, allowedTypes{value: t, pos: pos, names: names})
}
func (s *returnInference) wordResult(w *word) *typeConstraint {
	if result := s.results[w]; result != nil {
		if s.fallible[w] {
			return namedType("Result", result, s.families[w])
		}
		return result
	}
	return termFromType(w.result)
}

func (s *returnInference) prepareEffects(words []*word) {
	s.families = map[*word]*typeConstraint{}
	s.fallible = map[*word]bool{}
	for _, w := range words {
		if !hasPropagation(w.body) {
			continue
		}
		if isResult(w.result) {
			s.families[w] = namedType(resultFamily(w.result))
		} else {
			s.families[w] = freshType()
			s.fallible[w] = true
		}
	}
}
func (s *returnInference) recordEffect(family *typeConstraint, pos position) {
	if s.current == nil || s.families[s.current] == nil {
		return
	}
	prior, previousKnown := constraintType(s.families[s.current])
	next, nextKnown := constraintType(family)
	if previousKnown && nextKnown && prior != next && s.err == nil {
		s.err = errorAt(pos, CodeTryInput, "word %q propagates multiple error families %s and %s; handle or convert them explicitly", s.current.name, prior, next)
		return
	}
	s.equal(s.families[s.current], family, pos)
}

func (s *returnInference) join(a, b *typeConstraint, pos position) *typeConstraint {
	if a.root().name == string(neverType) {
		return b
	}
	if b.root().name == string(neverType) {
		return a
	}
	s.equal(a, b, pos)
	return a
}
func (s *returnInference) block(body []*statement, env inferScope) *typeConstraint {
	result := namedType(string(unitType))
	for _, stmt := range body {
		result = s.statement(stmt, env)
	}
	return result
}
func (s *returnInference) statement(stmt *statement, env inferScope) *typeConstraint {
	switch stmt.kind {
	case "let", "var":
		value := s.expr(stmt.expr, env, false)
		if stmt.annotation != "" {
			typ, ok := s.checker.normalizeType(stmt.annotation)
			if ok {
				s.equal(value, termFromType(typ), stmt.pos)
			}
		}
		env[stmt.name] = value
		if stmt.expr != nil && stmt.expr.kind == "name" && env["\x00error:"+stmt.expr.text] != nil {
			env["\x00error:"+stmt.name] = env["\x00error:"+stmt.expr.text]
		}
		return namedType(string(unitType))
	case "assign":
		value := s.expr(stmt.expr, env, false)
		if target := env[stmt.name]; target != nil {
			s.equal(target, value, stmt.pos)
		}
		return namedType(string(unitType))
	case "expression":
		return s.expr(stmt.expr, env, false)
	case "return":
		if stmt.expr != nil && stmt.expr.kind == "call" && stmt.expr.text == "err" {
			family := namedType("Error")
			for _, arg := range stmt.expr.args {
				s.expr(arg, env, false)
				if arg.kind == "name" && env["\x00error:"+arg.text] != nil {
					family = env["\x00error:"+arg.text]
				}
			}
			s.recordEffect(family, stmt.pos)
		} else if stmt.expr != nil && stmt.expr.kind == "call" && stmt.expr.text == "ok" && len(stmt.expr.args) == 1 {
			value := s.expr(stmt.expr.args[0], env, false)
			result := s.results[s.current]
			if isResult(s.current.result) {
				if inner, ok := resultInner(s.current.result); ok {
					result = termFromType(inner)
				}
			}
			s.equal(value, result, stmt.pos)
		} else {
			s.expr(stmt.expr, env, false)
		}
		return namedType(string(neverType))
	case "if":
		condition := s.expr(stmt.expr, env, false)
		if typ, ok := constraintType(condition); ok && typ != boolType && s.err == nil {
			err := withHint(errorAt(stmt.expr.pos, CodeConditionType, "%s", narrated(fmt.Sprintf("condition must have type Bool, found %s", typ), narrate(stmt.expr))), tryHint(typ))
			s.err = withSpan(err, stmt.expr)
		}
		s.equal(condition, namedType("Bool"), stmt.pos)
		left := s.block(stmt.body, copyInferScope(env))
		right := s.block(stmt.otherwise, copyInferScope(env))
		return s.join(left, right, stmt.pos)
	case "flow":
		value := s.expr(stmt.expr, env, false)
		for _, step := range stmt.steps {
			args := []*typeConstraint{value}
			for _, arg := range step.args {
				args = append(args, s.expr(arg, env, false))
			}
			value = s.termCall(step, args, env, step.propagate, 1)
			if step.propagate {
				var family *typeConstraint
				value, family = s.unwrapParts(value, step.pos)
				s.recordEffect(family, step.pos)
			}
		}
		return value
	case "for", "for-lines", "for-collection":
		first := s.expr(stmt.expr, env, false)
		nested := copyInferScope(env)
		switch stmt.kind {
		case "for":
			s.equal(first, namedType("Int"), stmt.pos)
			s.equal(s.expr(stmt.end, env, false), namedType("Int"), stmt.pos)
			nested[stmt.name] = namedType("Int")
		case "for-lines":
			s.equal(first, namedType("String"), stmt.pos)
			nested[stmt.name] = namedType("String")
		default:
			element := freshType()
			positionType := freshType()
			s.collectionParts(first, element, positionType, stmt.pos)
			if stmt.secondName == "" {
				nested[stmt.name] = element
			} else {
				nested[stmt.name] = positionType
				nested[stmt.secondName] = element
			}
		}
		s.block(stmt.body, nested)
		return namedType(string(unitType))
	}
	s.err = errorAt(stmt.pos, CodeUnsupportedCode, "unsupported inference statement %q", stmt.kind)
	return freshType()
}
func (s *returnInference) unwrapParts(result *typeConstraint, pos position) (*typeConstraint, *typeConstraint) {
	inner, family := freshType(), freshType()
	s.equal(result, namedType("Result", inner, family), pos)
	return inner, family
}
func (s *returnInference) infer(words []*word) error {
	for _, w := range words {
		if view := s.views[w]; view != nil {
			s.checker = view
		}
		s.current = w
		env := inferScope{}
		for i, param := range w.params {
			if terms := s.params[w]; i < len(terms) {
				env[param.name] = terms[i]
			} else {
				env[param.name] = termFromType(param.typ)
			}
			if param.defaultExpr != nil {
				s.equal(env[param.name], s.expr(param.defaultExpr, inferScope{}, false), param.pos)
			}
		}
		value := s.block(w.body, env)
		if value.root().name != string(neverType) {
			s.equal(s.results[w], value, w.pos)
		}
	}
	if len(s.entry) > 0 {
		s.current = nil
		if s.entryView != nil {
			s.checker = s.entryView
		}
		env := inferScope{}
		for scope := s.entryScope; scope != nil; scope = scope.parent {
			for name, b := range scope.bindings {
				if env[name] == nil {
					env[name] = termFromType(b.typ)
				}
			}
		}
		s.block(s.entry, env)
	}
	if s.err != nil {
		return s.err
	}
	for len(s.pending) > 0 {
		remaining := s.pending[:0]
		for _, rule := range s.pending {
			if !rule() {
				remaining = append(remaining, rule)
			}
		}
		if len(remaining) == len(s.pending) && s.err == nil {
			pos := position{}
			if s.current != nil {
				pos = s.current.pos
			} else if len(s.entry) > 0 {
				pos = s.entry[0].pos
			}
			return withHint(errorAt(pos, CodeWordResult, "cannot infer a type"), "add an explicit type annotation")
		}
		s.pending = remaining
		if s.err != nil {
			break
		}
	}
	if s.err != nil {
		return s.err
	}
	for _, rule := range s.allowed {
		concrete, ok := constraintType(rule.value)
		if !ok {
			return withHint(errorAt(rule.pos, CodeWordResult, "cannot infer an overloaded type"), "add an explicit type annotation")
		}
		found := false
		for _, name := range rule.names {
			if string(concrete) == name {
				found = true
				break
			}
		}
		if !found {
			return errorAt(rule.pos, CodeArgumentType, "type %s is not supported", concrete)
		}
	}
	for x, term := range s.partialTerms {
		if typ, ok := constraintType(term); ok {
			x.inferredPartial = typ
		}
	}
	for _, w := range words {
		for i := range w.params {
			param := &w.params[i]
			if param.typ != "" {
				continue
			}
			typ, ok := constraintType(s.params[w][i])
			if !ok || typ == neverType || containsResultType(typ) {
				return withHint(errorAt(param.pos, CodeArgumentType, "cannot infer type of parameter %q", param.name), "add an explicit parameter annotation")
			}
			if w.module != "" && !strings.HasPrefix(w.name, "_") && (privateTypeInModule(typ, w.module) || privateInferredType(typ, s.checker.types.all)) {
				return privateTypeLeak(param.pos, typ)
			}
			param.typ = typ
			param.inferredType = true
		}
		if w.result != "" && !s.fallible[w] {
			continue
		}
		result, ok := constraintType(s.results[w])
		if !ok || result == neverType {
			return withHint(errorAt(w.pos, CodeWordResult, "cannot infer return type of %q", w.name), "add an explicit return annotation")
		}
		if w.module != "" && !strings.HasPrefix(w.name, "_") && (privateTypeInModule(result, w.module) || privateInferredType(result, s.checker.types.all)) {
			return privateTypeLeak(w.pos, result)
		}
		if w.result == "" {
			w.inferredReturnType = true
		}
		w.result = result
		if s.fallible[w] {
			family, known := constraintType(s.families[w])
			if !known {
				return withHint(errorAt(w.pos, CodeTryInput, "cannot infer error family of %q", w.name), "add an explicit Result return annotation")
			}
			w.success, w.result, w.inferred = result, valueType("Result<"+string(result)+", "+string(family)+">"), true
		}
	}
	return nil
}
func (s *returnInference) fail(pos position, format string, args ...any) {
	if s.err == nil {
		s.err = errorAt(pos, CodeWordResult, fmt.Sprintf(format, args...))
	}
}

func (s *returnInference) expr(x *expression, env inferScope, checked bool) *typeConstraint {
	if x == nil {
		return namedType(string(unitType))
	}
	switch x.kind {
	case "flow-value":
		return termFromType(x.typ)
	case "hole":
		return freshType()
	case "int":
		return namedType("Int")
	case "string":
		return namedType("String")
	case "bool":
		return namedType("Bool")
	case "statement-value":
		return s.statement(x.statement, env)
	case "name":
		if choice, index := s.checker.resolveVariant(x.text); choice != nil && len(choice.variants[index].fields) == 0 {
			if choice.typeParam != "" {
				return namedType(choice.id(), freshType())
			}
			return namedType(choice.id())
		}
		if value := env[x.text]; value != nil {
			return value
		}
		parts := strings.Split(x.text, ".")
		if len(parts) > 1 && env[parts[0]] != nil {
			result := env[parts[0]]
			for _, field := range parts[1:] {
				result = s.recordField(result, field, x.pos)
			}
			return result
		}
		if x.bare {
			if word, _ := s.checker.resolveWord(x.text); word != nil || HostWord(x.text) {
				return s.call(x, nil, checked)
			}
		}
		if s.err == nil {
			var names []string
			for name := range env {
				names = append(names, name)
			}
			hint := s.checker.unknownNameHint(x.text, names)
			s.err = withSpan(withHint(errorAt(x.pos, CodeUnknownName, "unknown name %q", x.text), hint), x)
		}
		return freshType()
	case "list":
		element := freshType()
		for _, arg := range x.args {
			s.equal(element, s.expr(arg, env, false), arg.pos)
		}
		return namedType("List", element)
	case "map":
		key, value := freshType(), freshType()
		for i := 0; i+1 < len(x.args); i += 2 {
			s.equal(key, s.expr(x.args[i], env, false), x.args[i].pos)
			s.equal(value, s.expr(x.args[i+1], env, false), x.args[i+1].pos)
		}
		s.constrain(key, x.pos, "Int", "String", "Bool")
		return namedType("Map", key, value)
	case "call":
		if x.text == "error.set-reporter" && len(x.args) == 1 && x.args[0].kind == "name" {
			if word, private := s.checker.resolveWord(x.args[0].text); word != nil && private == nil && len(word.params) == 1 {
				param := termFromType(word.params[0].typ)
				if terms := s.params[word]; len(terms) == 1 {
					param = terms[0]
				}
				s.equal(param, namedType("Error"), x.pos)
				s.equal(s.wordResult(word), namedType("Unit"), x.pos)
			}
			return namedType("Unit")
		}
		args := make([]*typeConstraint, len(x.args))
		for i, arg := range x.args {
			args[i] = s.expr(arg, env, false)
		}
		return s.termCall(x, args, env, checked, 0)
	case "try":
		inner, family := s.unwrapParts(s.expr(x.left, env, true), x.pos)
		s.recordEffect(family, x.pos)
		return inner
	case "catch":
		inner, family := s.unwrapParts(s.expr(x.left, env, true), x.pos)
		nested := copyInferScope(env)
		if x.text != "" {
			nested[x.text] = namedType("Error")
			nested["\x00error:"+x.text] = family
		}
		fallback := s.block(x.handler, nested)
		if fallback.root().name != string(neverType) {
			s.equal(inner, fallback, x.pos)
		}
		return inner
	case "index":
		receiver := s.expr(x.left, env, false)
		indexes := make([]*typeConstraint, len(x.args))
		for i, arg := range x.args {
			indexes[i] = s.expr(arg, env, false)
		}
		return s.index(receiver, indexes, x.pos, checked, false)
	case "index-write":
		receiver := s.expr(x.left.left, env, false)
		indexes := make([]*typeConstraint, len(x.left.args))
		for i, arg := range x.left.args {
			indexes[i] = s.expr(arg, env, false)
		}
		element := s.index(receiver, indexes, x.pos, false, true)
		s.equal(element, s.expr(x.right, env, false), x.pos)
		result := freshType()
		resolve := func() bool {
			root := receiver.root()
			if root.name == "" {
				return false
			}
			if x.plainWrite || root.name == "Map" {
				s.equal(result, namedType("Unit"), x.pos)
			} else {
				s.equal(result, namedType("Result", namedType("Unit"), namedType("Error")), x.pos)
			}
			return true
		}
		if !resolve() {
			s.pending = append(s.pending, resolve)
		}
		return result
	case "unary":
		required := "Int"
		if x.text == "not" {
			required = "Bool"
		}
		s.equal(s.expr(x.left, env, false), namedType(required), x.pos)
		return namedType(required)
	case "binary":
		left, right := s.expr(x.left, env, false), s.expr(x.right, env, false)
		if leftType, leftOK := constraintType(left); leftOK {
			if rightType, rightOK := constraintType(right); rightOK && leftType != rightType && s.err == nil {
				nLeft, nRight := x.left, x.right
				if isResult(rightType) && !isResult(leftType) {
					nLeft, nRight = nRight, nLeft
				}
				s.err = valueError(x.pos, x, CodeOperandTypes, fmt.Sprintf("operator %q has incompatible operand types %s and %s", x.text, leftType, rightType), tryHint(leftType, rightType), nLeft, nRight)
			}
		}
		s.equal(left, right, x.pos)
		switch x.text {
		case "and", "or":
			s.equal(left, namedType("Bool"), x.pos)
			return namedType("Bool")
		case "==", "!=":
			s.constrain(left, x.pos, "Int", "String", "Bool")
			return namedType("Bool")
		case "<", "<=", ">", ">=":
			s.equal(left, namedType("Int"), x.pos)
			return namedType("Bool")
		case "+":
			s.constrain(left, x.pos, "Int", "String")
			return left
		default:
			s.equal(left, namedType("Int"), x.pos)
			return namedType("Int")
		}
	case "match":
		return s.match(x, env)
	}
	s.fail(x.pos, "unsupported inference expression %q", x.kind)
	return freshType()
}

func (s *returnInference) recordField(record *typeConstraint, name string, pos position) *typeConstraint {
	out := freshType()
	resolve := func() bool {
		root := record.root()
		if root.name == "" {
			return false
		}
		declaration := s.checker.types.all[root.name]
		if declaration == nil || declaration.choice || declaration.alias {
			s.fail(pos, "type %s has no field %q", root.name, name)
			return true
		}
		for _, field := range declaration.fields {
			if field.name == name {
				s.equal(out, termFromType(field.typ), pos)
				return true
			}
		}
		s.fail(pos, "type %s has no field %q", root.name, name)
		return true
	}
	if !resolve() {
		s.pending = append(s.pending, resolve)
	}
	return out
}

func (s *returnInference) index(receiver *typeConstraint, indexes []*typeConstraint, pos position, checked, write bool) *typeConstraint {
	element := freshType()
	result := freshType()
	resolve := func() bool {
		root := receiver.root()
		if root.name == "" {
			return false
		}
		switch root.name {
		case "List", "Array":
			if len(root.args) != 1 {
				s.fail(pos, "invalid collection type")
				return true
			}
			s.equal(element, root.args[0], pos)
			for _, idx := range indexes {
				s.equal(idx, namedType("Int"), pos)
			}
			if checked {
				s.equal(result, namedType("Result", element, namedType("Error")), pos)
			} else {
				s.equal(result, element, pos)
			}
		case "Map":
			if len(root.args) != 2 {
				s.fail(pos, "invalid map type")
				return true
			}
			for _, idx := range indexes {
				s.equal(idx, root.args[0], pos)
			}
			if write {
				s.equal(result, root.args[1], pos)
			} else {
				s.equal(result, namedType("Option", root.args[1]), pos)
			}
		default:
			s.fail(pos, "indexing requires a collection")
		}
		return true
	}
	if !resolve() {
		s.pending = append(s.pending, resolve)
	}
	return result
}

func (s *returnInference) collectionParts(collection, element, index *typeConstraint, pos position) {
	resolve := func() bool {
		root := collection.root()
		if root.name == "" {
			return false
		}
		switch root.name {
		case "List":
			if len(root.args) == 1 {
				s.equal(element, root.args[0], pos)
				s.equal(index, namedType("Int"), pos)
				return true
			}
		case "Array":
			if len(root.args) == 1 {
				s.equal(element, root.args[0], pos)
				s.equal(index, namedType("List", namedType("Int")), pos)
				return true
			}
		case "Map":
			if len(root.args) == 2 {
				s.equal(index, root.args[0], pos)
				s.equal(element, root.args[1], pos)
				return true
			}
		}
		s.fail(pos, "loop requires a collection")
		return true
	}
	if !resolve() {
		s.pending = append(s.pending, resolve)
	}
}

func (s *returnInference) match(x *expression, env inferScope) *typeConstraint {
	subject := s.expr(x.left, env, false)
	result := freshType()
	normal := false
	for _, arm := range x.arms {
		nested := copyInferScope(env)
		if arm.name != "_" {
			choice, index := s.checker.resolveVariant(arm.name)
			if choice == nil {
				s.fail(arm.pos, "unknown choice variant %q", arm.name)
				continue
			}
			param := freshType()
			expected := namedType(choice.id())
			if choice.typeParam != "" {
				expected = namedType(choice.id(), param)
			}
			s.equal(subject, expected, arm.pos)
			for i, name := range arm.names {
				if name != "_" && i < len(choice.variants[index].fields) {
					nested[name] = templateTerm(choice.variants[index].fields[i].typ, choice.typeParam, param)
				}
			}
		}
		branch := s.block(arm.body, nested)
		if branch.root().name != string(neverType) {
			normal = true
			s.equal(result, branch, arm.pos)
		}
	}
	if !normal {
		return namedType(string(neverType))
	}
	return result
}

// termCall carries callable terms through inference without creating a runtime
// Result: a partial's Result, if any, is its future invocation contract.
func (s *returnInference) termCall(x *expression, args []*typeConstraint, env inferScope, checked bool, offset int) *typeConstraint {
	var holes []*typeConstraint
	for i, arg := range x.args {
		if arg.kind == "hole" && i+offset < len(args) {
			holes = append(holes, args[i+offset])
		}
	}
	if len(holes) > 0 {
		checked = false
	}
	var result *typeConstraint
	if binding := env[x.text]; binding != nil {
		root := binding.root()
		word, _ := s.checker.resolveWord(x.text)
		if root.name == "@function" || (root.name == "" && word == nil && !HostWord(x.text)) {
			result = freshType()
			params := append(append([]*typeConstraint{}, args...), result)
			s.equal(binding, namedType("@function", params...), x.pos)
		} else if root.name == "" {
			// A later call site may establish that this binding is callable.
			// Until then, guessing the same-named word changes the program.
			result = freshType()
			view := s.checker
			deferred := result
			s.pending = append(s.pending, func() bool {
				resolved := binding.root()
				if resolved.name == "" {
					return false
				}
				if resolved.name == "@function" {
					params := append(append([]*typeConstraint{}, args...), deferred)
					s.equal(binding, namedType("@function", params...), x.pos)
				} else {
					previous := s.checker
					s.checker = view
					target := s.call(x, args, checked)
					s.checker = previous
					s.equal(deferred, target, x.pos)
				}
				return true
			})
		}
	}
	if result == nil {
		result = s.call(x, args, checked)
	}
	if len(holes) == 0 {
		return result
	}
	partial := namedType("@function", append(holes, result)...)
	if s.partialTerms == nil {
		s.partialTerms = map[*expression]*typeConstraint{}
	}
	s.partialTerms[x] = partial
	return partial
}

type inferenceSignature struct {
	params []valueType
	result valueType
}

var inferenceBuiltins = func() map[string]inferenceSignature {
	table := map[string]inferenceSignature{}
	add := func(names string, result valueType, params ...valueType) {
		for _, name := range strings.Fields(names) {
			table[name] = inferenceSignature{params: params, result: result}
		}
	}
	add("int.to-string", stringType, intType)
	add("text.reverse text.upper text.lower text.trim", stringType, stringType)
	add("text.to-int", intParseResultType, stringType)
	add("text.length text.line-count text.word-count", intType, stringType)
	add("text.contains", boolType, stringType, stringType)
	add("text.index-of", intType, stringType, stringType)
	add("text.slice", stringType, stringType, intType, intType)
	add("text.replace", stringType, stringType, stringType, stringType)
	add("buffer.new", bufferType)
	add("buffer.add-line", unitType, bufferType, stringType)
	add("buffer.line-count", intType, bufferType)
	add("buffer.line", stringBufferResultType, bufferType, intType)
	add("buffer.set-line", unitBufferResultType, bufferType, intType, stringType)
	add("buffer.to-text", stringType, bufferType)
	add("file.read-all", stringFileResultType, stringType)
	add("file.read-lines", bufferFileResultType, stringType)
	add("file.open", fileResultType, stringType)
	add("file.read-line", stringFileResultType, fileType)
	add("file.close", unitType, fileType)
	add("read-line", stringResultType, stringType)
	add("console.arg", stringResultType, intType)
	add("console.args-count", intType)
	add("console.exit", neverType, intType)
	add("console.fail", neverType, stringType)
	add("error.report", unitType, errorType)
	add("error.new", errorType, stringType, stringType)
	add("error.wrap", errorType, stringType, stringType, errorType)
	add("error.has-cause", boolType, errorType)
	add("error.cause", "Result<Error, Error>", errorType)
	add("error.code error.message error.operation error.file", stringType, errorType)
	add("error.line error.column", intType, errorType)
	add("error.detail", stringType, errorType, stringType)
	add("date.new", dateType, intType, intType, intType)
	add("time.new", timeType, intType, intType, intType)
	add("date.parse", "Result<Date, Error>", stringType)
	add("time.parse", "Result<Time, Error>", stringType)
	add("duration.parse", "Result<Duration, Error>", stringType)
	add("date.to-string", stringType, dateType)
	add("time.to-string", stringType, timeType)
	add("duration.to-string", stringType, durationType)
	add("date.add-days", dateType, dateType, intType)
	add("duration.new", durationType, intType)
	add("instant.now", instantType)
	add("instant.parse", "Result<Instant, Error>", stringType)
	add("instant.to-string", stringType, instantType)
	add("instant.add-duration", instantType, instantType, durationType)
	add("timezone.load", "Result<TimeZone, Error>", stringType)
	add("timezone.name", stringType, timezoneType)
	add("zoned.new", zonedType, dateType, timeType, timezoneType)
	add("zoned.to-string", stringType, zonedType)
	add("zoned.instant", instantType, zonedType)
	add("zoned.in-zone", zonedType, zonedType, timezoneType)
	add("zoned.add-days", zonedType, zonedType, intType)
	add("zoned.add-duration", zonedType, zonedType, durationType)
	return table
}()

func (s *returnInference) callArgument(x *expression, args []*typeConstraint, i int, expected *typeConstraint) {
	actual := args[i]
	if len(x.args) == len(args) && s.err == nil {
		found, foundOK := constraintType(actual)
		required, requiredOK := constraintType(expected)
		if foundOK && requiredOK && found != required {
			msg := fmt.Sprintf("argument %d of %s requires type %s, found %s", i+1, x.text, required, found)
			s.err = valueError(x.pos, x.args[i], CodeArgumentType, msg, tryHint(found), x.args[i])
		}
	}
	s.equal(actual, expected, x.pos)
}

func (s *returnInference) call(x *expression, args []*typeConstraint, checked bool) *typeConstraint {
	if choice, index := s.checker.resolveVariant(x.text); choice != nil {
		param := freshType()
		result := namedType(choice.id())
		if choice.typeParam != "" {
			result = namedType(choice.id(), param)
		}
		fields := choice.variants[index].fields
		for i, arg := range args {
			if i < len(fields) {
				s.equal(arg, templateTerm(fields[i].typ, choice.typeParam, param), x.pos)
			}
		}
		return result
	}
	if record := s.checker.types.visible[x.text]; record != nil && !record.alias && !record.choice {
		for i, arg := range args {
			index := i
			if len(x.argNames) > i {
				index = -1
				for j, field := range record.fields {
					if field.name == x.argNames[i] {
						index = j
						break
					}
				}
			}
			if index >= 0 && index < len(record.fields) {
				s.equal(arg, termFromType(record.fields[index].typ), x.pos)
			}
		}
		return namedType(record.id())
	}
	if word, private := s.checker.resolveWord(x.text); word != nil {
		if private != nil {
			if s.err == nil {
				s.err = withHint(withSpan(errorAt(x.pos, CodePrivateUse, "%q is private to module %q", word.name, private.name), x), fmt.Sprintf("remove the leading underscore in %s to export it", private.path))
			}
			return freshType()
		}
		for i := range args {
			if i < len(word.params) {
				expected := termFromType(word.params[i].typ)
				if terms := s.params[word]; i < len(terms) {
					expected = terms[i]
				}
				s.callArgument(x, args, i, expected)
			}
		}
		return s.wordResult(word)
	}
	if signature, ok := inferenceBuiltins[x.text]; ok {
		for i := range args {
			if i < len(signature.params) {
				s.callArgument(x, args, i, termFromType(signature.params[i]))
			}
		}
		return termFromType(signature.result)
	}
	switch x.text {
	case "print":
		if len(args) > 0 {
			s.constrain(args[0], x.pos, "Int", "String", "Bool")
		}
		return namedType("Unit")
	case "ok", "err":
		if s.current == nil {
			s.fail(x.pos, "%s requires a Result word context", x.text)
			return freshType()
		}
		result := s.wordResult(s.current)
		inner, family := freshType(), freshType()
		s.equal(result, namedType("Result", inner, family), x.pos)
		if len(args) > 0 {
			if x.text == "ok" {
				s.equal(args[0], inner, x.pos)
			} else {
				s.equal(args[0], namedType("Error"), x.pos)
			}
		}
		return result
	case "list.new":
		return namedType("List", freshType())
	case "list.length", "list.append":
		element := freshType()
		if len(args) > 0 {
			s.equal(args[0], namedType("List", element), x.pos)
		}
		if x.text == "list.length" {
			return namedType("Int")
		}
		if len(args) > 1 {
			s.equal(args[1], element, x.pos)
		}
		if checked {
			return namedType("Result", namedType("Unit"), namedType("Error"))
		}
		return namedType("Unit")
	case "array.new":
		element := freshType()
		if len(args) > 0 {
			s.equal(args[0], namedType("List", namedType("Int")), x.pos)
		}
		if len(args) > 1 {
			s.equal(args[1], element, x.pos)
		}
		result := namedType("Array", element)
		if checked {
			return namedType("Result", result, namedType("Error"))
		}
		return result
	case "array.resize", "array.shape", "array.rank", "array.length":
		element := freshType()
		if len(args) > 0 {
			s.equal(args[0], namedType("Array", element), x.pos)
		}
		switch x.text {
		case "array.resize":
			if len(args) > 1 {
				s.equal(args[1], namedType("List", namedType("Int")), x.pos)
			}
			if len(args) > 2 {
				s.equal(args[2], element, x.pos)
			}
			if checked {
				return namedType("Result", namedType("Unit"), namedType("Error"))
			}
			return namedType("Unit")
		case "array.shape":
			return namedType("List", namedType("Int"))
		default:
			return namedType("Int")
		}
	case "map.length", "map.contains", "map.remove", "map.put", "map.into":
		key, value := freshType(), freshType()
		collection := namedType("Map", key, value)
		if len(args) > 0 {
			s.equal(args[0], collection, x.pos)
		}
		if x.text == "map.into" {
			if len(args) > 1 {
				s.equal(args[1], collection, x.pos)
			}
			return collection
		}
		if len(args) > 1 {
			s.equal(args[1], key, x.pos)
		}
		if x.text == "map.put" && len(args) > 2 {
			s.equal(args[2], value, x.pos)
		}
		switch x.text {
		case "map.length":
			return namedType("Int")
		case "map.contains":
			return namedType("Bool")
		case "map.put":
			return collection
		default:
			return namedType("Unit")
		}
	case "json.decode":
		if len(args) > 0 {
			s.equal(args[0], namedType("String"), x.pos)
		}
		target := freshType()
		if len(args) > 1 {
			s.equal(target, args[1], x.pos)
		}
		return namedType("Result", target, namedType("Error"))
	case "json.encode":
		return namedType("Result", namedType("String"), namedType("Error"))
	}
	if s.err == nil {
		s.err = withSpan(withHint(errorAt(x.pos, CodeUnknownWord, "unknown word %q", x.text), suggestWordHint(x.text, s.checker.wordNames())), x)
	}
	return freshType()
}
