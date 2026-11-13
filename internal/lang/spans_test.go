package lang

import "testing"

// TestExpressionsCarryEndPositions pins exclusive end positions per
// expression: end is one past the last rune, on the same line, measured in
// source runes (not decoded width).
func TestExpressionsCarryEndPositions(t *testing.T) {
	parse := func(t *testing.T, source string) *Program {
		t.Helper()
		program, err := parse("bad.zhex", source)
		if err != nil {
			t.Fatalf("parse %q: %v", source, err)
		}
		return program
	}
	cases := []struct {
		name   string
		source string
		node   func(p *Program) *expression
		posCol int
		endCol int
	}{
		{"argument name", "print(a-b)\n", func(p *Program) *expression { return p.body[0].expr.args[0] }, 7, 10},
		{"inner call", "print(int.to-string(2))\n", func(p *Program) *expression { return p.body[0].expr.args[0] }, 7, 23},
		{"binary", "print(1 + \"a\")\n", func(p *Program) *expression { return p.body[0].expr.args[0] }, 9, 14},
		{"unary", "print(not 1)\n", func(p *Program) *expression { return p.body[0].expr.args[0] }, 7, 12},
		{"try", "let n = text.to-int(\"1\") try\n", func(p *Program) *expression { return p.body[0].expr }, 26, 29},
		{"string source width", "print(\"a\\tb\")\n", func(p *Program) *expression { return p.body[0].expr.args[0] }, 7, 13},
		{"negated literal", "print(-9223372036854775808)\n", func(p *Program) *expression { return p.body[0].expr.args[0] }, 7, 27},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x := tc.node(parse(t, tc.source))
			if x.pos.line != 1 || x.end.line != 1 {
				t.Fatalf("expected same-line span, got line %d..%d", x.pos.line, x.end.line)
			}
			if x.pos.column != tc.posCol || x.end.column != tc.endCol {
				t.Fatalf("span = %d..%d, want %d..%d", x.pos.column, x.end.column, tc.posCol, tc.endCol)
			}
		})
	}
}
