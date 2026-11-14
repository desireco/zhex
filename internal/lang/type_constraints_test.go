package lang

import "testing"

func TestTypeConstraintsUnifyNestedTypes(t *testing.T) {
	a, b := &typeConstraint{}, &typeConstraint{}
	left := &typeConstraint{name: "Map", args: []*typeConstraint{{name: "String"}, {name: "List", args: []*typeConstraint{a}}}}
	right := &typeConstraint{name: "Map", args: []*typeConstraint{{name: "String"}, {name: "List", args: []*typeConstraint{b}}}}
	if !unifyTypes(left, right) || !unifyTypes(b, &typeConstraint{name: "Int"}) {
		t.Fatal("valid nested constraints failed")
	}
	got, ok := constraintType(left)
	if !ok || got != "Map<String, List<Int>>" {
		t.Fatalf("got %q, %v", got, ok)
	}
	if unifyTypes(a, &typeConstraint{name: "Bool"}) {
		t.Fatal("accepted contradictory types")
	}
}

func TestTypeConstraintsRejectCyclesAndUnresolvedValues(t *testing.T) {
	a := &typeConstraint{}
	if unifyTypes(a, &typeConstraint{name: "List", args: []*typeConstraint{a}}) {
		t.Fatal("accepted recursive type")
	}
	if _, ok := constraintType(a); ok {
		t.Fatal("unresolved type was treated as concrete")
	}
}

func TestTypeConstraintsBoundDepthAndHandleLongVariableChains(t *testing.T) {
	variables := make([]*typeConstraint, 10000)
	for i := range variables {
		variables[i] = &typeConstraint{}
	}
	for i := 0; i+1 < len(variables); i++ {
		if !unifyTypes(variables[i], variables[i+1]) {
			t.Fatal("variable chain rejected")
		}
	}
	if !unifyTypes(variables[len(variables)-1], &typeConstraint{name: "Int"}) {
		t.Fatal("could not resolve chain")
	}
	if got, ok := constraintType(variables[0]); !ok || got != "Int" {
		t.Fatalf("got %q %v", got, ok)
	}
	deep := &typeConstraint{name: "Int"}
	for i := 0; i < 258; i++ {
		deep = &typeConstraint{name: "List", args: []*typeConstraint{deep}}
	}
	if unifyTypes(&typeConstraint{}, deep) {
		t.Fatal("accepted excessive type depth")
	}
}

func TestTypeConstraintsBoundSharedExpansion(t *testing.T) {
	value := &typeConstraint{name: "Int"}
	for i := 0; i < 40; i++ {
		value = &typeConstraint{name: "Pair", args: []*typeConstraint{value, value}}
	}
	if !unifyTypes(&typeConstraint{}, value) {
		t.Fatal("shared acyclic constraint rejected")
	}
	if _, ok := constraintType(value); ok {
		t.Fatal("expanded type exceeded safety limit")
	}
}
