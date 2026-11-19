package zhexrt

// The panicking arithmetic the generated code emits: every operation the
// interpreter treats as a hard runtime diagnostic (integer overflow,
// division by zero) panics with a positioned *Error, which Run renders —
// never a raw Go panic escaping. Operation names mirror the interpreter's
// (the operator symbol).

func Add(pos Pos, a, b Int) Int {
	sum, overflow := AddOvf(a, b)
	if overflow {
		FailOp(pos, "integer-overflow", "+", "integer overflow")
	}
	return sum
}

func Sub(pos Pos, a, b Int) Int {
	diff, overflow := SubOvf(a, b)
	if overflow {
		FailOp(pos, "integer-overflow", "-", "integer overflow")
	}
	return diff
}

func Mul(pos Pos, a, b Int) Int {
	product, overflow := MulOvf(a, b)
	if overflow {
		FailOp(pos, "integer-overflow", "*", "integer overflow")
	}
	return product
}

func Neg(pos Pos, a Int) Int {
	n, ok := NegOvf(a)
	if !ok {
		FailOp(pos, "integer-overflow", "-", "integer overflow")
	}
	return n
}

func Div(pos Pos, a, b Int) Int {
	if b.IsZero() {
		FailOp(pos, "division-zero", "/", "division or remainder by zero")
	}
	if a == Min && b == I64(-1) {
		FailOp(pos, "integer-overflow", "/", "integer overflow")
	}
	q, _ := DivMod(a, b)
	return q
}

func Rem(pos Pos, a, b Int) Int {
	if b.IsZero() {
		FailOp(pos, "division-zero", "%", "division or remainder by zero")
	}
	if a == Min && b == I64(-1) {
		return I64(0)
	}
	_, r := DivMod(a, b)
	return r
}

// Word-call depth: the interpreter's 256-frame limit (#96) applies to
// every user-word call, so every generated word enters and leaves. The
// limit failure is a hard diagnostic (never catchable), hence a panic.
const MaxDepth = 256

var depth int

func EnterWord(pos Pos, name string) {
	if depth >= MaxDepth {
		panic(&Error{Pos: pos, Code: "depth-limit", Operation: name, Message: "word call depth limit of 256 exceeded"})
	}
	depth++
}

func LeaveWord() { depth-- }
