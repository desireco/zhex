package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestAnnotatedBindingsCheckDeclaredType(t *testing.T) {
	source := "let count: Int = 2\nvar name: String = \"Ada\"\nname = \"Grace\"\nprint(count)\nprint(name)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\nGrace\n" {
		t.Fatalf("output %q", out.String())
	}
	for _, bad := range []string{"let n: Int = \"wrong\"\n", "var n: Bool = 1\n", "let n: Unknown = 1\n"} {
		if _, err := Compile("test.zhex", bad); err == nil {
			t.Errorf("accepted invalid annotation: %q", bad)
		}
	}
}
