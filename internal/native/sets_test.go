package native

import "testing"

// TestNativeSetWords pins the six set words (#235a): the native build must
// agree with the interpreter byte-for-byte, including the print braces and
// quoted String members.
func TestNativeSetWords(t *testing.T) {
	diffCase(t, "sets",
		`var s: Set<Int> = set.new
var i = 0
while i < 200
    set.add(s, i % 37)
    i = i + 1
print(set.length(s))
print(set.contains(s, 36))
set.remove(s, 36)
print(set.contains(s, 36))
print(set.length(s))
var total = 0
let xs = set.values(s)
var j = 0
while j < list.length(xs)
    total = total + xs[j]
    j = j + 1
print(total)
print(s)

var names: Set<String> = set.new
set.add(names, "Ada")
set.add(names, "Bo")
set.add(names, "Ada")
print(set.length(names))
let ys = set.values(names)
print(ys[0])
print(ys[1])
print(names)

var flags: Set<Bool> = set.new
set.add(flags, true)
set.add(flags, false)
set.add(flags, true)
print(set.length(flags))
print(set.contains(flags, false))
print(flags)

let empty: Set<Int> = set.new
print(set.length(empty))
print(set.contains(empty, 1))
print(empty)
`, "")
}
