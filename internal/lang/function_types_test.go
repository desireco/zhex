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

func TestFunctionTypesAcceptFutureResultButNotStoredResult(t *testing.T) {
	valid := "word accept(op: (Int) -> Result<Int, Error>) -> Unit\n    print(1)\nlet callbacks: List<(Int) -> Result<Int, Error>> = []\n"
	if _, err := Compile("test.zhex", valid); err != nil {
		t.Fatal(err)
	}
	invalid := "word accept(op: (Result<Int, Error>) -> Int) -> Unit\n    print(1)\n"
	if _, err := Compile("test.zhex", invalid); err == nil {
		t.Fatal("accepted function parameter containing a stored Result")
	}
}

func TestGenericChoiceCanDescribeFallibleFunction(t *testing.T) {
	source := "type Holder<T> = choice\n    Ready(action: (T) -> Result<Int, Error>)\n"
	if _, err := Compile("test.zhex", source); err != nil {
		t.Fatal(err)
	}
}
