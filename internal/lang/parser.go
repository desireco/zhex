package lang

import (
	"strconv"
	"strings"
)

type parser struct {
	tokens       []token
	index, depth int
}

func parse(file, source string) (program *Program, err error) {
	tokens, err := lex(file, source)
	if err != nil {
		return nil, err
	}
	// Syntax failures unwind recursive descent; programming errors are never swallowed.
	defer func() {
		if caught := recover(); caught != nil {
			if d, ok := caught.(*diagnostic); ok {
				program = nil
				err = d
			} else {
				panic(caught)
			}
		}
	}()
	p := parser{tokens: tokens}
	program = &Program{}
	for !p.at("eof") {
		if p.at("type") {
			program.records = append(program.records, p.parseRecord())
		} else if p.at("word") {
			program.words = append(program.words, p.parseWord())
		} else if p.at("use") && p.depth == 0 {
			program.uses = append(program.uses, p.parseUse())
		} else {
			program.body = append(program.body, p.parseStatement())
		}
	}
	return program, nil
}

func (p *parser) current() token { return p.tokens[p.index] }
func (p *parser) at(kind string) bool {
	t := p.current()
	return t.kind == kind || t.kind == "name" && t.text == kind
}
func (p *parser) take() token { t := p.current(); p.index++; return t }
func (p *parser) accept(kind string) bool {
	if p.at(kind) {
		p.take()
		return true
	}
	return false
}
func (p *parser) require(kind string) token {
	if !p.at(kind) {
		panic(errorAt(p.current().pos, CodeExpectedToken, "expected %s, found %q", kind, p.current().text))
	}
	return p.take()
}

// Reserved reports whether name is a Zhex keyword.
func Reserved(name string) bool {
	switch name {
	case "word", "type", "record", "choice", "match", "case", "use", "let", "var", "if", "elif", "else", "for", "in", "flow", "try", "catch", "return", "and", "or", "not", "true", "false":
		return true
	}
	return false
}

func (p *parser) name() token {
	t := p.require("name")
	if Reserved(t.text) {
		panic(errorAt(t.pos, CodeExpectedName, "expected a name, found keyword %q", t.text))
	}
	return t
}

// parseUse reads a top-level import: use math. Module names are simple —
// the dot belongs to qualified calls, not to use.
func (p *parser) parseUse() use {
	t := p.take()
	n := p.name()
	if strings.Contains(n.text, ".") {
		panic(withHint(errorAt(n.pos, CodeExpectedName, "expected a module name, found %q", n.text),
			"module names cannot contain dots; use the module's name, as in use math"))
	}
	p.require("newline")
	return use{name: n.text, pos: t.pos, end: n.end}
}

func (p *parser) parseType() valueType {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 256 {
		panic(errorAt(p.current().pos, CodeNestingLimit, "syntax nesting limit exceeded"))
	}
	if p.accept("(") {
		var args []valueType
		if !p.at(")") {
			for {
				args = append(args, p.parseType())
				if !p.accept(",") {
					break
				}
			}
		}
		p.require(")")
		p.require("->")
		return functionType(args, p.parseType())
	}
	t := p.require("name")
	if p.accept("<") {
		args := []string{string(p.parseType())}
		for p.accept(",") {
			args = append(args, string(p.parseType()))
		}
		p.require(">")
		return valueType(t.text + "<" + strings.Join(args, ", ") + ">")
	}
	return valueType(t.text)
}

func (p *parser) parseRecord() *recordType {
	p.require("type")
	name := p.name()
	param := ""
	if p.accept("<") {
		param = p.name().text
		p.require(">")
	}
	p.require("=")
	if p.accept("alias") {
		if param != "" {
			panic(errorAt(name.pos, CodeUnknownType, "generic aliases are not supported"))
		}
		target := p.parseType()
		p.require("newline")
		return &recordType{name: name.text, pos: name.pos, alias: true, aliasTarget: target, aliasSource: target}
	}
	choice := p.accept("choice")
	if param != "" && !choice {
		panic(errorAt(name.pos, CodeUnknownType, "type parameters require a choice declaration"))
	}
	if !choice {
		p.require("record")
	}
	p.require("newline")
	if !p.at("indent") {
		panic(errorAt(p.current().pos, CodeExpectedIndent, "expected an indented block"))
	}
	p.require("indent")
	result := &recordType{name: name.text, pos: name.pos, choice: choice, typeParam: param}
	for !p.at("dedent") && !p.at("eof") {
		field := p.name()
		if choice {
			variant := choiceVariant{name: field.text, pos: field.pos}
			if p.accept("(") {
				if !p.at(")") {
					for {
						n := p.name()
						p.require(":")
						variant.fields = append(variant.fields, parameter{name: n.text, pos: n.pos, typ: p.parseType()})
						if !p.accept(",") {
							break
						}
					}
				}
				p.require(")")
			}
			result.variants = append(result.variants, variant)
		} else {
			p.require(":")
			f := parameter{name: field.text, pos: field.pos, typ: p.parseType()}
			if p.accept("=") {
				f.defaultExpr = p.expression(0)
			} else if len(result.fields) > 0 && result.fields[len(result.fields)-1].defaultExpr != nil {
				panic(errorAt(field.pos, CodeArgumentCount, "required fields must precede fields with defaults"))
			}
			result.fields = append(result.fields, f)
		}
		p.require("newline")
	}
	p.require("dedent")
	if len(result.fields) == 0 && len(result.variants) == 0 {
		panic(errorAt(name.pos, CodeEmptyBlock, "record requires at least one field"))
	}
	return result
}

func (p *parser) parseWord() *word {
	p.require("word")
	t := p.name()
	w := &word{name: t.text, pos: t.pos}
	parenthesized := p.accept("(")
	if (parenthesized && !p.at(")")) || (!parenthesized && p.current().kind == "name" && !p.at("newline")) {
		for {
			n := p.name()
			param := parameter{name: n.text, pos: n.pos}
			if p.accept(":") {
				param.typ = p.parseType()
			}
			if p.accept("=") {
				param.defaultExpr = p.expression(0)
			} else if len(w.params) > 0 && w.params[len(w.params)-1].defaultExpr != nil {
				panic(errorAt(n.pos, CodeArgumentCount, "required parameters must precede parameters with defaults"))
			}
			w.params = append(w.params, param)
			if !p.accept(",") {
				break
			}
		}
	}
	if parenthesized {
		p.require(")")
	}
	if !p.accept("->") {
		w.body = p.block()
		return w
	}
	if p.at("Result") {
		p.take()
		p.require("<")
		inner := p.parseType()
		p.require(",")
		family := p.require("name")
		switch family.text {
		case "InputError", "ParseError", "FileError", "BufferError", "Error":
		default:
			panic(errorAt(family.pos, CodeUnknownType, "unknown error type %q", family.text))
		}
		p.require(">")
		w.result = valueType("Result<" + string(inner) + ", " + family.text + ">")
	} else {
		w.result = p.parseType()
	}
	w.body = p.block()
	return w
}

func (p *parser) block() []*statement {
	p.require("newline")
	if !p.at("indent") {
		panic(errorAt(p.current().pos, CodeExpectedIndent, "expected an indented block"))
	}
	p.take()
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 256 {
		panic(errorAt(p.current().pos, CodeNestingLimit, "syntax nesting limit exceeded"))
	}
	var body []*statement
	for !p.at("dedent") && !p.at("eof") {
		body = append(body, p.parseStatement())
	}
	p.require("dedent")
	if len(body) == 0 {
		panic(errorAt(p.current().pos, CodeEmptyBlock, "expected a nonempty indented block"))
	}
	return body
}

func (p *parser) parseStatement() *statement {
	t := p.current()
	if p.at("use") {
		err := withHint(errorAt(t.pos, CodeUseLocation, "use is only allowed at the top level"),
			"move the use statement to the top level of the file")
		if next := p.tokens[p.index+1]; next.kind == "name" {
			err = withSpanRange(err, t.pos, next.end)
		}
		panic(err)
	}
	s := &statement{pos: t.pos}
	switch {
	case p.at("let") || p.at("var"):
		s.kind = p.take().text
		s.name = p.name().text
		if p.accept(":") {
			s.annotation = p.parseType()
		}
		p.require("=")
		s.expr = p.statementExpression(false)
	case p.accept("return"):
		s.kind = "return"
		s.expr = p.expression(0)
		p.require("newline")
	case p.at("if"):
		return p.parseIf()
	case p.accept("for"):
		s.kind = "for"
		s.name = p.name().text
		if p.accept(",") {
			s.secondName = p.name().text
		}
		p.require("in")
		if s.secondName == "" && p.accept("range") {
			p.require("(")
			s.expr = p.expression(0)
			p.require(",")
			s.end = p.expression(0)
			p.require(")")
		} else if s.secondName == "" && p.accept("file.lines") {
			s.kind = "for-lines"
			p.require("(")
			s.expr = p.expression(0)
			p.require(")")
		} else {
			s.kind = "for-collection"
			s.expr = p.expression(0)
		}
		s.body = p.block()
	case p.accept("flow"):
		s.kind = "flow"
		s.expr = p.wholeExpression()
		p.require("newline")
		if !p.at("indent") {
			panic(errorAt(p.current().pos, CodeExpectedIndent, "expected an indented flow"))
		}
		p.take()
		for !p.at("dedent") && !p.at("eof") {
			step := p.name()
			x := &expression{kind: "call", text: step.text, pos: step.pos, end: step.end}
			if p.accept("(") {
				p.callArguments(x)
			} else if p.bareArgumentStart() && p.current().pos.line == x.end.line && p.current().pos.column > x.end.column {
				x.args = p.bareArguments()
				last, guard := trailingBareTry(x.args[len(x.args)-1])
				if guard != nil {
					x.args[len(x.args)-1] = last
					x.propagate = true
				}
			}
			x.propagate = p.accept("try") || x.propagate
			s.steps = append(s.steps, x)
			p.require("newline")
		}
		p.require("dedent")
		if len(s.steps) == 0 {
			panic(errorAt(s.pos, CodeFlowStep, "flow requires a step"))
		}
	case t.kind == "name" && !Reserved(t.text) && p.tokens[p.index+1].kind == "=":
		s.kind = "assign"
		s.name = p.take().text
		p.require("=")
		if p.at("if") || p.at("match") || p.at("flow") {
			s.expr = p.statementExpression(false)
		} else {
			s.expr = p.wholeExpression()
			p.require("newline")
		}
	default:
		s.kind = "expression"
		s.expr = p.statementExpression(true)
	}
	return s
}

// Bare calls occur only at a complete statement-value boundary. Argument
// expressions keep the existing precedence and nesting rules.
func (p *parser) bareArgumentStart() bool {
	t := p.current()
	return t.kind == "number" || t.kind == "string" || t.kind == "name" && (!Reserved(t.text) || t.text == "true" || t.text == "false" || t.text == "not")
}

// trailingBareTry extracts an ungrouped postfix try at the far right of the
// final argument. Parentheses preserve an intentionally guarded argument.
func trailingBareTry(x *expression) (*expression, *expression) {
	if x == nil || x.grouped {
		return x, nil
	}
	if x.kind == "try" {
		return x.left, x
	}
	if x.kind == "unary" {
		left, guard := trailingBareTry(x.left)
		if guard != nil {
			x.left = left
			x.end = left.end
			return x, guard
		}
	}
	if x.kind == "binary" {
		right, guard := trailingBareTry(x.right)
		if guard != nil {
			x.right = right
			x.end = right.end
			return x, guard
		}
	}
	return x, nil
}

func (p *parser) argument() *expression {
	if p.at("_") {
		next := p.tokens[p.index+1]
		if next.kind == "," || next.kind == ")" || next.kind == "newline" || next.text == "try" || next.text == "catch" {
			t := p.take()
			return &expression{kind: "hole", text: "_", pos: t.pos, end: t.end}
		}
	}
	return p.expression(0)
}
func (p *parser) bareArguments() []*expression {
	var args []*expression
	for {
		args = append(args, p.argument())
		if !p.accept(",") {
			break
		}
	}
	return args
}
func (p *parser) wholeExpression() *expression {
	x := p.expression(0)
	if x.kind == "name" && p.bareArgumentStart() && p.current().pos.line == x.end.line && p.current().pos.column > x.end.column {
		x.kind = "call"
		x.args = p.bareArguments()
		last, guard := trailingBareTry(x.args[len(x.args)-1])
		x.args[len(x.args)-1] = last
		x.end = last.end
		if guard == nil && p.at("try") {
			t := p.take()
			guard = &expression{pos: t.pos, end: t.end}
		}
		if guard != nil {
			return p.checkDepth(&expression{kind: "try", pos: guard.pos, end: guard.end, left: p.checkDepth(x)})
		}
		return p.checkDepth(x)
	}
	if x.kind == "name" {
		x.bare = true
	}
	if x.kind == "try" && x.left != nil && x.left.kind == "name" {
		x.left.bare = true
	}
	return x
}

// statementExpression accepts an indented catch as the whole statement value.
func (p *parser) statementExpression(allowAssignment bool) *expression {
	if p.at("match") {
		return p.parseMatch()
	}
	if !allowAssignment && (p.at("if") || p.at("flow")) {
		t := p.current()
		return &expression{kind: "statement-value", pos: t.pos, end: t.end, statement: p.parseStatement()}
	}
	x := p.wholeExpression()
	if allowAssignment && p.accept("=") {
		if x.kind != "index" {
			panic(errorAt(x.pos, CodeExpectedExpression, "assignment requires an indexed collection"))
		}
		right := p.expression(0)
		p.require("newline")
		return p.checkDepth(&expression{kind: "index-write", pos: x.pos, end: right.end, left: x, right: right, plainWrite: true})
	}
	if p.at("catch") {
		t := p.take()
		name := ""
		if p.current().kind == "name" {
			name = p.name().text
		}
		return &expression{kind: "catch", pos: t.pos, end: t.end, text: name, left: x, handler: p.block()}
	}
	p.require("newline")
	return x
}

func (p *parser) parseMatch() *expression {
	t := p.require("match")
	x := &expression{kind: "match", pos: t.pos, left: p.expression(0)}
	p.require("newline")
	if !p.at("indent") {
		panic(errorAt(p.current().pos, CodeExpectedIndent, "expected an indented match"))
	}
	p.require("indent")
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 256 {
		panic(errorAt(t.pos, CodeNestingLimit, "syntax nesting limit exceeded"))
	}
	for !p.at("dedent") && !p.at("eof") {
		c := p.require("case")
		name := p.name()
		arm := matchArm{pos: c.pos, name: name.text}
		if p.accept("(") {
			if !p.at(")") {
				for {
					arm.names = append(arm.names, p.name().text)
					if !p.accept(",") {
						break
					}
				}
			}
			p.require(")")
		}
		arm.body = p.block()
		x.arms = append(x.arms, arm)
	}
	p.require("dedent")
	if len(x.arms) == 0 {
		panic(errorAt(t.pos, CodeEmptyBlock, "match requires at least one case"))
	}
	return x
}

func (p *parser) parseIf() *statement {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 256 {
		panic(errorAt(p.current().pos, CodeNestingLimit, "syntax nesting limit exceeded"))
	}
	t := p.take()
	s := &statement{kind: "if", pos: t.pos, expr: p.expression(0)}
	s.body = p.block()
	if p.at("elif") {
		s.otherwise = []*statement{p.parseIf()}
	} else if p.accept("else") {
		s.otherwise = p.block()
	}
	return s
}

func precedence(op string) int {
	switch op {
	case "or":
		return 1
	case "and":
		return 2
	case "==", "!=":
		return 3
	case "<", "<=", ">", ">=":
		return 4
	case "+", "-":
		return 5
	case "*", "/", "%":
		return 6
	}
	return -1
}

func (p *parser) expression(minimum int) *expression {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 256 {
		panic(errorAt(p.current().pos, CodeNestingLimit, "syntax nesting limit exceeded"))
	}
	left := p.checkDepth(p.unary())
	for {
		t := p.current()
		level := precedence(t.text)
		if level < minimum {
			break
		}
		p.take()
		right := p.expression(level + 1)
		left = p.checkDepth(&expression{kind: "binary", text: t.text, pos: t.pos, end: right.end, left: left, right: right})
	}
	return left
}

func (p *parser) checkDepth(x *expression) *expression {
	depth := 0
	for _, child := range append([]*expression{x.left, x.right}, x.args...) {
		if child != nil && child.depth > depth {
			depth = child.depth
		}
	}
	x.depth = depth + 1
	if x.depth > 256 {
		panic(errorAt(x.pos, CodeNestingLimit, "syntax nesting limit exceeded"))
	}
	return x
}

func (p *parser) unary() *expression {
	if p.at("-") || p.at("not") {
		t := p.take()
		if t.text == "-" && p.at("number") {
			n := p.take()
			n.text = "-" + n.text
			n.pos = t.pos
			return p.number(n)
		}
		// Route through expression to include the nesting limit for repeated unary operators.
		u := &expression{kind: "unary", text: t.text, pos: t.pos, left: p.expression(7)}
		u.end = u.left.end
		return u
	}
	x := p.primary()
	for p.at("[") {
		t := p.take()
		index := &expression{kind: "index", pos: t.pos, left: x}
		if !p.at("]") {
			for {
				index.args = append(index.args, p.expression(0))
				if !p.accept(",") {
					break
				}
			}
		}
		index.end = p.require("]").end
		x = p.checkDepth(index)
	}
	for p.at("try") {
		t := p.take()
		x = p.checkDepth(&expression{kind: "try", pos: t.pos, end: t.end, left: x})
	}
	return x
}

func (p *parser) number(t token) *expression {
	n, err := strconv.ParseInt(t.text, 10, 64)
	if err != nil {
		panic(errorAt(t.pos, CodeIntegerLiteralRange, "integer literal is outside the signed 64-bit range"))
	}
	return &expression{kind: "int", number: n, pos: t.pos, end: t.end}
}

func (p *parser) primary() *expression {
	t := p.current()
	if p.accept("{") {
		x := &expression{kind: "map", pos: t.pos}
		if !p.at("}") {
			for {
				x.args = append(x.args, p.expression(0))
				p.require(":")
				x.args = append(x.args, p.expression(0))
				if !p.accept(",") {
					break
				}
			}
		}
		x.end = p.require("}").end
		return p.checkDepth(x)
	}
	if p.accept("[") {
		x := &expression{kind: "list", pos: t.pos}
		if !p.at("]") {
			for {
				x.args = append(x.args, p.expression(0))
				if !p.accept(",") {
					break
				}
			}
		}
		x.end = p.require("]").end
		return x
	}
	if p.accept("(") {
		x := p.expression(0)
		if p.accept("=") {
			if x.kind != "index" {
				panic(errorAt(x.pos, CodeExpectedExpression, "indexed assignment requires a collection index"))
			}
			right := p.expression(0)
			end := p.require(")")
			return p.checkDepth(&expression{kind: "index-write", pos: x.pos, end: end.end, left: x, right: right})
		}
		p.require(")")
		x.grouped = true
		return x
	}
	if t.kind == "number" {
		p.take()
		return p.number(t)
	}
	if t.kind == "string" {
		p.take()
		return &expression{kind: "string", text: t.text, pos: t.pos, end: t.end}
	}
	if p.at("true") || p.at("false") {
		p.take()
		return &expression{kind: "bool", text: t.text, pos: t.pos, end: t.end}
	}
	if t.kind == "name" && !Reserved(t.text) {
		p.take()
		x := &expression{kind: "name", text: t.text, pos: t.pos, end: t.end}
		if p.accept("(") {
			x.kind = "call"
			p.callArguments(x)
		}
		return x
	}
	panic(errorAt(t.pos, CodeExpectedExpression, "expected an expression, found %q", t.text))
}

func (p *parser) callArguments(x *expression) {
	var names []string
	anyNamed := false
	if !p.at(")") {
		for {
			name := ""
			if p.current().kind == "name" && p.tokens[p.index+1].kind == ":" {
				name = p.name().text
				p.require(":")
				anyNamed = true
			}
			names = append(names, name)
			x.args = append(x.args, p.argument())
			if !p.accept(",") {
				break
			}
		}
	}
	if anyNamed {
		x.argNames = names
	}
	x.end = p.require(")").end
}
