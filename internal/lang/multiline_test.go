package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestMultilineDelimitersPreserveExpressions(t *testing.T) {
	source := `word add(
    a: Int,
    b: Int
) -> Int
    a + b
let values: List<Int> = [
  1,
  # comment inside a delimiter

  add(
      2,
      3
  )
]
let scores = {
    "a": values[
      1
    ]
}
print(values[
    1
])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n" {
		t.Fatal(out.String())
	}
}

func TestMultilineDelimiterErrorsAndLocations(t *testing.T) {
	for _, source := range []string{"let xs = [\n 1,\n", "let xs = [\n 1)\n", "let xs = [\n\t1\n]\n", "let xs = [\n 1,\n]\n"} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted invalid multiline source: %q", source)
		}
	}
	_, err := Compile("test.zhex", "print(\n    missing\n)\n")
	if err == nil || !strings.Contains(err.Error(), "test.zhex:2:5:") {
		t.Fatalf("wrong source location: %v", err)
	}
}

func TestMultilineFlowCatchAndResultEffects(t *testing.T) {
	source := `word parse(raw: String) -> Int
    let value = text.to-int(
        raw
    ) try
    value
let n = parse(
    "bad"
) catch problem
    4
flow "Ada"
    text.replace(
        "A",
        "E"
    )
    print
print(n)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Eda\n4\n" {
		t.Fatal(out.String())
	}
}

func TestMultilineDelimiterNestingLimit(t *testing.T) {
	source := "print(" + strings.Repeat("(", 257) + "1" + strings.Repeat(")", 257) + ")\n"
	if _, err := Compile("test.zhex", source); err == nil || !strings.Contains(err.Error(), "nesting limit") {
		t.Fatalf("missing limit: %v", err)
	}
}
