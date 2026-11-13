package lang

type scope struct {
	parent   *scope
	bindings map[string]*binding
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent, bindings: make(map[string]*binding)}
}
func (s *scope) find(name string) *binding {
	for current := s; current != nil; current = current.parent {
		if b := current.bindings[name]; b != nil {
			return b
		}
	}
	return nil
}
func (s *scope) names() []string {
	var out []string
	for current := s; current != nil; current = current.parent {
		for name := range current.bindings {
			out = append(out, name)
		}
	}
	return out
}
func (s *scope) define(pos position, name string, typ valueType, mutable bool) (*binding, error) {
	if s.bindings[name] != nil {
		return nil, errorAt(pos, CodeDuplicateName, "name %q is already defined in this scope", name)
	}
	b := &binding{name, typ, mutable}
	s.bindings[name] = b
	return b, nil
}

type checker struct{ words map[string]*word }

var hostWords = []string{"print", "int.to-string", "text.reverse", "text.to-int", "read-line", "range"}

// HostWord reports whether name is a word provided by the host, not the program.
func HostWord(name string) bool {
	for _, word := range hostWords {
		if word == name {
			return true
		}
	}
	return false
}

func (c *checker) wordNames() []string {
	names := append([]string{}, hostWords...)
	for name := range c.words {
		names = append(names, name)
	}
	return names
}

// Compile parses and checks every word before the program can perform any I/O.
func Compile(file, source string) (*Program, error) {
	p, err := parse(file, source)
	if err != nil {
		return nil, err
	}
	p.source = source
	c := checker{words: make(map[string]*word)}
	for _, w := range p.words {
		if c.words[w.name] != nil || HostWord(w.name) {
			return nil, errorAt(w.pos, CodeDuplicateWord, "word %q is already defined", w.name)
		}
		c.words[w.name] = w
	}
	for _, w := range p.words {
		env := newScope(nil)
		for i := range w.params {
			param := &w.params[i]
			param.binding, err = env.define(param.pos, param.name, param.typ, false)
			if err != nil {
				return nil, err
			}
		}
		result, err := c.block(w.body, env)
		if err != nil {
			return nil, err
		}
		if result != w.result {
			return nil, errorAt(w.pos, CodeWordResult, "word %q return type is %s; declared %s", w.name, result, w.result)
		}
	}
	if _, err = c.block(p.body, newScope(nil)); err != nil {
		return nil, err
	}
	return p, nil
}

func (c *checker) block(body []*statement, env *scope) (valueType, error) {
	typ := unitType
	for _, s := range body {
		var err error
		typ, err = c.statement(s, env)
		if err != nil {
			return "", err
		}
	}
	return typ, nil
}

func (c *checker) statement(s *statement, env *scope) (valueType, error) {
	typ, err := c.expression(s.expr, env)
	if err != nil {
		return "", err
	}
	switch s.kind {
	case "expression":
		return typ, nil
	case "let", "var":
		s.binding, err = env.define(s.pos, s.name, typ, s.kind == "var")
		return unitType, err
	case "assign":
		s.binding = env.find(s.name)
		if s.binding == nil {
			return "", withHint(errorAt(s.pos, CodeUnknownName, "unknown name %q", s.name), nameHint(s.name, append(env.names(), c.wordNames()...)))
		}
		if !s.binding.mutable {
			return "", errorAt(s.pos, CodeImmutableName, "cannot assign to immutable name %q", s.name)
		}
		if typ != s.binding.typ {
			return "", errorAt(s.pos, CodeAssignmentType, "assignment type %s does not match %s", typ, s.binding.typ)
		}
		return unitType, nil
	case "if":
		if typ != boolType {
			return "", withHint(errorAt(s.expr.pos, CodeConditionType, "condition must have type Bool, found %s", typ), tryHint(typ))
		}
		left, err := c.block(s.body, newScope(env))
		if err != nil {
			return "", err
		}
		right, err := c.block(s.otherwise, newScope(env))
		if err != nil {
			return "", err
		}
		if left != right {
			return "", errorAt(s.pos, CodeBranchType, "branch result types must match: %s and %s", left, right)
		}
		return left, nil
	case "for":
		end, err := c.expression(s.end, env)
		if err != nil {
			return "", err
		}
		if typ != intType || end != intType {
			return "", errorAt(s.pos, CodeRangeType, "range bounds must have type Int")
		}
		inner := newScope(env)
		s.binding, err = inner.define(s.pos, s.name, intType, false)
		if err != nil {
			return "", err
		}
		_, err = c.block(s.body, inner)
		return unitType, err
	case "flow":
		for _, step := range s.steps {
			typ, err = c.call(step, []valueType{typ})
			if err != nil {
				return "", err
			}
		}
		return typ, nil
	}
	return "", errorAt(s.pos, CodeUnsupportedCode, "unsupported statement %q", s.kind)
}

func (c *checker) expression(x *expression, env *scope) (valueType, error) {
	var typ valueType
	switch x.kind {
	case "int":
		typ = intType
	case "bool":
		typ = boolType
	case "string":
		typ = stringType
	case "name":
		x.binding = env.find(x.text)
		if x.binding == nil {
			return "", withHint(errorAt(x.pos, CodeUnknownName, "unknown name %q", x.text), nameHint(x.text, append(env.names(), c.wordNames()...)))
		}
		typ = x.binding.typ
	case "call":
		args := make([]valueType, len(x.args))
		for i, arg := range x.args {
			var err error
			args[i], err = c.expression(arg, env)
			if err != nil {
				return "", err
			}
		}
		return c.call(x, args)
	case "try":
		inner, err := c.expression(x.left, env)
		if err != nil {
			return "", err
		}
		switch inner {
		case intResultType, intParseResultType:
			typ = intType
		case stringResultType:
			typ = stringType
		default:
			return "", errorAt(x.pos, CodeTryInput, "try requires an input Result, found %s", inner)
		}
	case "unary":
		inner, err := c.expression(x.left, env)
		if err != nil {
			return "", err
		}
		typ = intType
		if x.text == "not" {
			typ = boolType
		}
		if inner != typ {
			return "", withHint(errorAt(x.pos, CodeUnaryType, "operator %q requires type %s, found %s", x.text, typ, inner), tryHint(inner))
		}
	case "binary":
		left, err := c.expression(x.left, env)
		if err != nil {
			return "", err
		}
		right, err := c.expression(x.right, env)
		if err != nil {
			return "", err
		}
		if left != right {
			return "", withHint(errorAt(x.pos, CodeOperandTypes, "operator %q has incompatible operand types %s and %s", x.text, left, right), tryHint(left, right))
		}
		typ = left
		switch x.text {
		case "and", "or":
			if left != boolType {
				return "", withHint(errorAt(x.pos, CodeBoolOperand, "operator %q requires type Bool", x.text), tryHint(left))
			}
		case "==", "!=":
			if left != intType && left != stringType && left != boolType {
				return "", withHint(errorAt(x.pos, CodeCompareType, "cannot compare type %s", left), tryHint(left))
			}
			typ = boolType
		case "<", "<=", ">", ">=":
			if left != intType {
				return "", withHint(errorAt(x.pos, CodeOrderingType, "ordering requires type Int"), tryHint(left))
			}
			typ = boolType
		case "+":
			if left != intType && left != stringType {
				return "", withHint(errorAt(x.pos, CodeAdditionType, "addition requires type Int or String"), tryHint(left))
			}
		default:
			if left != intType {
				return "", withHint(errorAt(x.pos, CodeArithmeticType, "arithmetic requires type Int"), tryHint(left))
			}
		}
	default:
		return "", errorAt(x.pos, CodeUnsupportedCode, "unsupported expression %q", x.kind)
	}
	x.typ = typ
	return typ, nil
}

func (c *checker) call(x *expression, args []valueType) (valueType, error) {
	var params []valueType
	var result valueType
	if w := c.words[x.text]; w != nil {
		x.word = w
		result = w.result
		for _, p := range w.params {
			params = append(params, p.typ)
		}
	} else {
		switch x.text {
		case "print":
			if len(args) != 1 {
				return "", errorAt(x.pos, CodeArgumentCount, "print requires 1 argument, found %d", len(args))
			}
			if args[0] != intType && args[0] != stringType && args[0] != boolType {
				return "", withHint(errorAt(x.pos, CodePrintType, "print does not support type %s", args[0]), tryHint(args[0]))
			}
			params = args
			result = unitType
		case "int.to-string":
			params = []valueType{intType}
			result = stringType
		case "text.reverse":
			params = []valueType{stringType}
			result = stringType
		case "text.to-int":
			params = []valueType{stringType}
			result = intParseResultType
		case "read-line":
			params = []valueType{stringType}
			result = stringResultType
		case "range":
			return "", errorAt(x.pos, CodeRangeContext, "range is only available in a for loop")
		default:
			return "", withHint(errorAt(x.pos, CodeUnknownWord, "unknown word %q", x.text), suggestHint(x.text, c.wordNames()))
		}
	}
	if len(args) != len(params) {
		return "", errorAt(x.pos, CodeArgumentCount, "%s requires %d arguments, found %d", x.text, len(params), len(args))
	}
	for i, typ := range args {
		if typ != params[i] {
			return "", withHint(errorAt(x.pos, CodeArgumentType, "argument %d of %s requires type %s, found %s", i+1, x.text, params[i], typ), tryHint(typ))
		}
	}
	x.typ = result
	if len(args) > 0 {
		x.inputType = args[0]
	}
	return result, nil
}
