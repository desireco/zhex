package lang

import "strings"

// PublicWord is one public word of a compiled file, with whether its doc
// carries a doctest — the zhex test --gaps sweep maps these onto test-file
// references.
type PublicWord struct {
	Name      string
	Doctested bool
}

// PublicWords compiles the file like RunDoctests and lists its public words
// (bare names; leading-underscore words stay private) with their doctest
// presence. A module's words are listed by sweeping the module file itself.
func PublicWords(file, source string, resolve func(name string) (ModuleSource, bool)) ([]PublicWord, error) {
	p, err := CompileModules(file, source, resolve)
	if err != nil {
		return nil, err
	}
	var words []PublicWord
	for _, w := range p.words {
		if strings.HasPrefix(w.name, "_") {
			continue
		}
		words = append(words, PublicWord{Name: w.name, Doctested: len(doctests(w)) != 0})
	}
	return words, nil
}

// TestFailure is the diagnostic a failed .test.zhex file reports through
// --report=json when its own output carries no diagnostic: a plain-program
// failure has no position, so line and column stay 0.
func TestFailure(file, message, hint string) error {
	return &diagnostic{pos: position{file: file}, code: CodeTestFailed, severity: "error", message: message, hint: hint}
}
