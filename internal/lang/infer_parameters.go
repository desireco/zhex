package lang

import "strings"

type inferenceSession struct {
	words   map[string]*word
	records map[string]*recordType
	scope   *scope
	uses    []string
}

// inferModuleParameters solves one graph before dependency-first checking, so
// callers in importing files can constrain an imported word's parameters.
func inferModuleParameters(p *Program, modules map[string]*module, order []*module, session *inferenceSession) error {
	needed := false
	for _, m := range order {
		for _, w := range m.order {
			for _, param := range w.params {
				if param.typ == "" {
					needed = true
				}
			}
		}
	}
	if !needed {
		return nil
	}
	all := collectRecords(modules)
	if session != nil {
		for _, record := range session.records {
			all[record.id()] = record
		}
	}
	views := map[*word]*checker{}
	var words []*word
	prepare := func(c *checker, declarations []*word) error {
		for _, w := range declarations {
			if w.result != "" {
				if w.module != "" && !strings.HasPrefix(w.name, "_") && c.rawPrivateType(w.result, w.module) {
					return privateTypeLeak(w.pos, w.result)
				}
				typ, ok := c.normalizeType(w.result)
				if !ok {
					return c.typeError(w.pos, w.result)
				}
				w.result = typ
			}
			for i := range w.params {
				param := &w.params[i]
				if param.typ == "" {
					continue
				}
				if w.module != "" && !strings.HasPrefix(w.name, "_") && c.rawPrivateType(param.typ, w.module) {
					return privateTypeLeak(param.pos, param.typ)
				}
				typ, ok := c.normalizeType(param.typ)
				if !ok {
					return c.typeError(param.pos, param.typ)
				}
				if containsResultType(typ) {
					return errorAt(param.pos, CodeUnhandledResult, "a Result cannot be passed as a word parameter")
				}
				param.typ = typ
			}
			words = append(words, w)
			views[w] = c
		}
		return nil
	}
	for _, m := range order {
		c := &checker{module: m.name, words: m.words, uses: m.uses, modules: modules, types: namedTypeRegistry{all: all}}
		c.types.visible = c.importedRecords(m.uses)
		if err := c.registerRecords(m.recordOrder); err != nil {
			return err
		}
		if err := c.checkAdopted(m.uses); err != nil {
			return err
		}
		if err := prepare(c, m.order); err != nil {
			return err
		}
	}
	c := &checker{words: map[string]*word{}, uses: useNames(p), modules: modules, types: namedTypeRegistry{all: all}}
	if session != nil {
		c.uses = session.uses
		for name, w := range session.words {
			c.words[name] = w
		}
		for _, w := range p.words {
			delete(c.words, w.name)
		}
	}
	c.types.visible = c.importedRecords(c.uses)
	if session != nil {
		for name, record := range session.records {
			c.types.visible[name] = record
		}
	}
	if err := c.registerRecords(p.records); err != nil {
		return err
	}
	if err := c.register(p.words); err != nil {
		return err
	}
	if session == nil {
		if err := c.checkAdopted(c.uses); err != nil {
			return err
		}
	}
	if err := prepare(c, p.words); err != nil {
		return err
	}
	effects := make(map[*word]map[string]bool, len(words))
	for _, w := range words {
		effects[w] = map[string]bool{}
	}
	for changed := true; changed; {
		changed = false
		for _, w := range words {
			found := map[string]bool{}
			views[w].effectBlock(w.body, map[string]string{}, effects, found)
			for family := range found {
				if !effects[w][family] {
					effects[w][family] = true
					changed = true
				}
			}
		}
	}
	solver := &returnInference{checker: c, views: views, entryView: c, entry: p.body, results: map[*word]*typeConstraint{}, params: map[*word][]*typeConstraint{}, effects: effects}
	if session != nil {
		solver.entryScope = session.scope
	}
	for _, w := range words {
		if w.result == "" {
			solver.results[w] = freshType()
		} else {
			solver.results[w] = termFromType(w.result)
		}
		for _, param := range w.params {
			if param.typ == "" {
				solver.params[w] = append(solver.params[w], freshType())
			} else {
				solver.params[w] = append(solver.params[w], termFromType(param.typ))
			}
		}
	}
	return solver.infer(words)
}
