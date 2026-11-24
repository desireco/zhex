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

// ShiftLeft is int.shift-left (#231): the mathematical a·2^n, failing the
// existing integer-overflow code exactly like * does when that value leaves
// the signed range. Zero shifted by any count stays zero and never fails;
// a negative count is the int-shift-count runtime fault in every build.
// 64-bit builds (wasm, the browser engine) fence counts of 64 or more with
// int-shift-width before this body runs.
func ShiftLeft(pos Pos, a, n Int) Int {
	if n.Negative() {
		FailOp(pos, "int-shift-count", "int.shift-left", "shift count is negative")
	}
	if a.IsZero() {
		return Int{}
	}
	k, inWidth := shiftCount(n)
	if !inWidth {
		FailOp(pos, "integer-overflow", "int.shift-left", "integer overflow")
	}
	r := shl(a, k)
	// Exact when the truncating shift reverses under the arithmetic one;
	// any bit lost to the width or the sign edge makes it differ.
	if shr(r, k) != a {
		FailOp(pos, "integer-overflow", "int.shift-left", "integer overflow")
	}
	return r
}

// ShiftRight is int.shift-right (#231): the arithmetic, sign-keeping shift —
// mathematically floor(a/2^n) — which never overflows. Counts past the
// width clamp to the mathematical result: 0, or −1 for negative values.
// A negative count is the int-shift-count runtime fault in every build.
func ShiftRight(pos Pos, a, n Int) Int {
	if n.Negative() {
		FailOp(pos, "int-shift-count", "int.shift-right", "shift count is negative")
	}
	k, inWidth := shiftCount(n)
	if !inWidth {
		if a.Negative() {
			return Int{Lo: ^uint64(0), Hi: -1}
		}
		return Int{}
	}
	return shr(a, k)
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
