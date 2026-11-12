package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestArithmetic(t *testing.T) {
	var out bytes.Buffer
	err := Run("arithmetic.zhex", "print(2 + 3 * 4)\n", strings.NewReader(""), &out)
	if err != nil || out.String() != "14\n" {
		t.Fatalf("output %q, error %v", out.String(), err)
	}
}

func TestInvalidProgramHasNoEffects(t *testing.T) {
	var out bytes.Buffer
	err := Run("invalid.zhex", "print(1)\nprint(\"text\" + 3)\n", strings.NewReader(""), &out)
	if err == nil || out.Len() != 0 || !strings.Contains(err.Error(), "invalid.zhex:2:") {
		t.Fatalf("output %q, error %v", out.String(), err)
	}
}
