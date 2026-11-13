package lang

import "testing"

func FuzzCompile(f *testing.F) {
	for _, source := range []string{"", "print(1)\n", "if true\n    print(1)\n", "word square(n: Int) -> Int\n    n * n\n", "let n = text.to-int(\"N: \") try\n", "print(\"\\\"\")\n", "\xff"} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 16384 {
			t.Skip()
		}
		_, _ = Compile("fuzz.zhex", source)
	})
}
