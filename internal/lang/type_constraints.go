package lang

import "strings"

// typeConstraint is either an unbound variable (empty name), a link to a
// unified term, or a constructor with zero or more type arguments.
type typeConstraint struct {
	name string
	args []*typeConstraint
	link *typeConstraint
}

func (t *typeConstraint) root() *typeConstraint {
	root := t
	for root.link != nil {
		root = root.link
	}
	for t.link != nil {
		next := t.link
		t.link = root
		t = next
	}
	return root
}

func occursInType(needle, value *typeConstraint) bool {
	seen := map[*typeConstraint]int{}
	var visit func(*typeConstraint, int) bool
	visit = func(value *typeConstraint, depth int) bool {
		if depth > 256 {
			return true
		}
		value = value.root()
		if needle == value {
			return true
		}
		if prior, ok := seen[value]; ok && prior >= depth {
			return false
		}
		seen[value] = depth
		for _, arg := range value.args {
			if visit(arg, depth+1) {
				return true
			}
		}
		return false
	}
	return visit(value, 0)
}

func unifyTypes(a, b *typeConstraint) bool {
	return unifyTypesDepth(a, b, 0, map[[2]*typeConstraint]int{})
}

func unifyTypesDepth(a, b *typeConstraint, depth int, seen map[[2]*typeConstraint]int) bool {
	if depth > 256 {
		return false
	}
	a, b = a.root(), b.root()
	if a == b {
		return true
	}
	if a.name == "" {
		if occursInType(a, b) {
			return false
		}
		a.link = b
		return true
	}
	if b.name == "" {
		return unifyTypesDepth(b, a, depth, seen)
	}
	if a.name != b.name || len(a.args) != len(b.args) {
		return false
	}
	pair := [2]*typeConstraint{a, b}
	if prior, ok := seen[pair]; ok && prior >= depth {
		return true
	}
	seen[pair] = depth
	for i := range a.args {
		if !unifyTypesDepth(a.args[i], b.args[i], depth+1, seen) {
			return false
		}
	}
	return true
}

func constraintType(t *typeConstraint) (valueType, bool) {
	type rendered struct {
		text   string
		height int
	}
	memo := map[*typeConstraint]rendered{}
	var render func(*typeConstraint, int) (string, bool)
	render = func(t *typeConstraint, depth int) (string, bool) {
		if depth > 256 {
			return "", false
		}
		t = t.root()
		if cached, ok := memo[t]; ok {
			if depth+cached.height > 256 {
				return "", false
			}
			return cached.text, true
		}
		if t.name == "" {
			return "", false
		}
		if len(t.args) == 0 {
			memo[t] = rendered{text: t.name}
			return t.name, true
		}
		args := make([]string, len(t.args))
		size, height := len(t.name)+2, 0
		for i, arg := range t.args {
			var ok bool
			args[i], ok = render(arg, depth+1)
			if !ok {
				return "", false
			}
			size += len(args[i])
			if i > 0 {
				size += 2
			}
			if size > 1<<20 {
				return "", false
			}
			if h := memo[arg.root()].height + 1; h > height {
				height = h
			}
		}
		text := t.name + "<" + strings.Join(args, ", ") + ">"
		if t.name == "@function" {
			text = "(" + strings.Join(args[:len(args)-1], ", ") + ") -> " + args[len(args)-1]
		}
		memo[t] = rendered{text: text, height: height}
		return text, true
	}
	value, ok := render(t, 0)
	return valueType(value), ok
}
