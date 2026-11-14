package lang

import "testing"

func TestFunctionTypePartsRespectNestedTypes(t *testing.T) {
	typ := valueType("(Map<String, Int>, (String) -> Result<Int, Error>) -> List<Int>")
	args, result, ok := functionTypeParts(typ)
	if !ok || len(args) != 2 || args[0] != "Map<String, Int>" || args[1] != "(String) -> Result<Int, Error>" || result != "List<Int>" {
		t.Fatalf("%v %q %t", args, result, ok)
	}
	base, nested, ok := typeApplication("List<(String, Int) -> Result<Int, Error>>")
	if !ok || base != "List" || len(nested) != 1 || nested[0] != "(String, Int) -> Result<Int, Error>" {
		t.Fatalf("%s %v %t", base, nested, ok)
	}
	inner, family, ok := resultParts("Result<(String, Int) -> Int, Error>")
	if !ok || inner != "(String, Int) -> Int" || family != "Error" {
		t.Fatalf("%s %s %t", inner, family, ok)
	}
}
