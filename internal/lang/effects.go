package lang

import "sort"

// inferWordEffects computes escaping Result families before checking callers.
// Module imports are acyclic and checked in dependency order; the fixed point
// below covers forward and recursive calls within each file.
func (c *checker) inferWordEffects(words []*word) error {
	effects := make(map[*word]map[string]bool, len(words))
	for _, w := range words {
		effects[w] = map[string]bool{}
	}
	for changed := true; changed; {
		changed = false
		for _, w := range words {
			found := map[string]bool{}
			c.effectBlock(w.body, map[string]string{}, effects, found)
			for family := range found {
				if !effects[w][family] {
					effects[w][family] = true
					changed = true
				}
			}
		}
	}
	for _, w := range words {
		if isResult(w.result) {
			continue
		}
		families := make([]string, 0, len(effects[w]))
		for family := range effects[w] {
			families = append(families, family)
		}
		sort.Strings(families)
		if len(families) > 1 {
			return errorAt(w.pos, CodeTryInput, "word %q propagates multiple error families %v; handle or convert them explicitly", w.name, families)
		}
		if len(families) == 1 {
			w.success = w.result
			w.result = valueType("Result<" + string(w.success) + ", " + families[0] + ">")
			w.inferred = true
		}
	}
	return nil
}

func (c *checker) effectResult(x *expression, env map[string]string, effects map[*word]map[string]bool) []string {
	if x == nil {
		return nil
	}
	switch x.kind {
	case "name":
		if f := env[x.text]; f != "" {
			return []string{f}
		}
	case "index", "index-write":
		return []string{"Error"}
	case "call":
		switch x.text {
		case "read-line", "console.arg":
			return []string{"InputError"}
		case "text.to-int":
			return []string{"ParseError"}
		case "file.read-all", "file.read-lines", "file.open", "file.read-line":
			return []string{"FileError"}
		case "buffer.line", "buffer.set-line":
			return []string{"BufferError"}
		case "list.append", "array.new", "array.resize", "error.cause":
			return []string{"Error"}
		}
		if w, _ := c.resolveWord(x.text); w != nil {
			if family := resultFamily(w.result); family != "" {
				return []string{family}
			}
			var out []string
			for family := range effects[w] {
				out = append(out, family)
			}
			return out
		}
	}
	return nil
}
func copyEffectsEnv(env map[string]string) map[string]string {
	out := make(map[string]string, len(env))
	for k, v := range env {
		out[k] = v
	}
	return out
}
func (c *checker) effectBlock(body []*statement, env map[string]string, effects map[*word]map[string]bool, found map[string]bool) {
	for _, s := range body {
		c.effectExpr(s.expr, env, effects, found)
		c.effectExpr(s.end, env, effects, found)
		if s.kind == "return" && s.expr != nil && s.expr.kind == "call" && s.expr.text == "err" {
			family := "Error"
			if len(s.expr.args) == 1 && s.expr.args[0].kind == "name" {
				if f, ok := env[s.expr.args[0].text]; ok {
					family = f
				}
			}
			if family != "?" {
				found[family] = true
			}
		}
		if s.kind == "let" || s.kind == "var" {
			fs := c.effectResult(s.expr, env, effects)
			if len(fs) == 1 {
				env[s.name] = fs[0]
			} else {
				delete(env, s.name)
			}
		}
		c.effectBlock(s.body, copyEffectsEnv(env), effects, found)
		c.effectBlock(s.otherwise, copyEffectsEnv(env), effects, found)
		for _, step := range s.steps {
			if step.propagate {
				for _, family := range c.effectResult(step, env, effects) {
					if family != "?" {
						found[family] = true
					}
				}
			}
		}
	}
}
func (c *checker) effectExpr(x *expression, env map[string]string, effects map[*word]map[string]bool, found map[string]bool) {
	if x == nil {
		return
	}
	if x.kind == "try" {
		for _, family := range c.effectResult(x.left, env, effects) {
			if family != "?" {
				found[family] = true
			}
		}
	}
	c.effectExpr(x.left, env, effects, found)
	c.effectExpr(x.right, env, effects, found)
	for _, arg := range x.args {
		c.effectExpr(arg, env, effects, found)
	}
	handlerEnv := copyEffectsEnv(env)
	if x.kind == "catch" && x.text != "" {
		families := c.effectResult(x.left, env, effects)
		handlerEnv[x.text] = "?"
		if len(families) == 1 {
			handlerEnv[x.text] = families[0]
		}
	}
	c.effectBlock(x.handler, handlerEnv, effects, found)
	for _, arm := range x.arms {
		c.effectBlock(arm.body, copyEffectsEnv(env), effects, found)
	}
}
