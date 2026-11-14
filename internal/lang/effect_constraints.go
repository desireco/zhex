package lang

// hasPropagation is syntactic: a try always escapes its enclosing word, while
// catch handles its protected Result without propagation.
func hasPropagation(body []*statement) bool {
	for _, stmt := range body {
		if stmt.kind == "return" && stmt.expr != nil && stmt.expr.kind == "call" && stmt.expr.text == "err" {
			return true
		}
		if expressionHasPropagation(stmt.expr) || expressionHasPropagation(stmt.end) || hasPropagation(stmt.body) || hasPropagation(stmt.otherwise) {
			return true
		}
		for _, step := range stmt.steps {
			if step.propagate || expressionHasPropagation(step) {
				return true
			}
		}
	}
	return false
}
func expressionHasPropagation(x *expression) bool {
	if x == nil {
		return false
	}
	if x.kind == "try" {
		return true
	}
	if x.statement != nil && hasPropagation([]*statement{x.statement}) {
		return true
	}
	if expressionHasPropagation(x.left) || expressionHasPropagation(x.right) || hasPropagation(x.handler) {
		return true
	}
	for _, arg := range x.args {
		if expressionHasPropagation(arg) {
			return true
		}
	}
	for _, arm := range x.arms {
		if hasPropagation(arm.body) {
			return true
		}
	}
	return false
}
func typeHasFunction(typ valueType) bool {
	base, args, ok := typeApplication(typ)
	if !ok {
		return false
	}
	if base == "@function" {
		return true
	}
	for _, arg := range args {
		if typeHasFunction(arg) {
			return true
		}
	}
	return false
}
func expressionHasFunction(x *expression) bool {
	if x == nil {
		return false
	}
	if x.kind == "hole" {
		return true
	}
	if x.statement != nil && blockHasFunction([]*statement{x.statement}) {
		return true
	}
	if expressionHasFunction(x.left) || expressionHasFunction(x.right) || blockHasFunction(x.handler) {
		return true
	}
	for _, arg := range x.args {
		if expressionHasFunction(arg) {
			return true
		}
	}
	for _, arm := range x.arms {
		if blockHasFunction(arm.body) {
			return true
		}
	}
	return false
}
func blockHasFunction(body []*statement) bool {
	for _, stmt := range body {
		if typeHasFunction(stmt.annotation) || expressionHasFunction(stmt.expr) || expressionHasFunction(stmt.end) || blockHasFunction(stmt.body) || blockHasFunction(stmt.otherwise) {
			return true
		}
		for _, step := range stmt.steps {
			if expressionHasFunction(step) {
				return true
			}
		}
	}
	return false
}
func (c *checker) needsFunctionInference(words []*word) bool {
	for _, w := range words {
		if typeHasFunction(w.result) || blockHasFunction(w.body) {
			return true
		}
		for _, p := range w.params {
			if typeHasFunction(p.typ) {
				return true
			}
		}
	}
	for _, m := range c.modules {
		for _, w := range m.order {
			if typeHasFunction(w.result) {
				return true
			}
			for _, p := range w.params {
				if typeHasFunction(p.typ) {
					return true
				}
			}
		}
	}
	for _, record := range c.types.all {
		for _, field := range record.fields {
			if typeHasFunction(field.typ) {
				return true
			}
		}
		for _, variant := range record.variants {
			for _, field := range variant.fields {
				if typeHasFunction(field.typ) {
					return true
				}
			}
		}
	}
	return false
}
