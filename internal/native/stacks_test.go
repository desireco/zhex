package native

import "testing"

// TestNativeStackWords pins the eight stack words (#236a1): the native
// build must agree with the interpreter byte-for-byte — the LIFO order,
// the Forth ops, the with-block threading, the bracket print with the top
// last, and the unhandled underflow's stack-range diagnostic.
func TestNativeStackWords(t *testing.T) {
	diffCase(t, "stack",
		`var s: Stack<Int> = stack.new
stack.push(s, 10)
stack.push(s, 20)
stack.push(s, 30)
print(stack.length(s))
print(stack.peek(s))
print(stack.pop(s))
print(stack.peek(s))
print(stack.length(s))
stack.swap(s)
print(stack.pop(s))
stack.dup(s)
print(stack.length(s))
stack.drop(s)
print(stack.pop(s))
print(stack.length(s))
print(s)

var words: Stack<String> = stack.new
stack.push(words, "a")
stack.push(words, "b")
print(stack.pop(words))
print(stack.peek(words))
stack.dup(words)
print(stack.pop(words))
print(stack.pop(words))
print(stack.length(words))
print(words)

var flags: Stack<Bool> = stack.new
stack.push(flags, true)
stack.push(flags, false)
print(stack.length(flags))
print(stack.pop(flags))
stack.push(flags, true)
stack.swap(flags)
print(stack.peek(flags))
stack.dup(flags)
stack.drop(flags)
print(stack.length(flags))
print(flags)

var drained = 0
var nums: Stack<Int> = stack.new
var i = 0
while i < 100
    stack.push(nums, i)
    i = i + 1
while stack.length(nums) > 0
    drained = drained + stack.pop(nums)
print(drained)

let top = flow nums
with stack
    push 7
    dup
    swap
    drop
    pop
print(top)
print(stack.length(nums))

let empty: Stack<Int> = stack.new
print(stack.length(empty))
stack.pop(empty)
`, "")
}
