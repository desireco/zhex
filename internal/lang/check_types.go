package lang

import (
	"fmt"
	"strings"
)

// namedTypeRegistry separates the declarations visible in the current file
// from the full graph needed to check imported field types and cycles.
var builtinOption = &recordType{name: "Option", choice: true, typeParam: "T", variants: []choiceVariant{{name: "None"}, {name: "Some", fields: []parameter{{name: "value", typ: "T"}}}}}

type namedTypeRegistry struct {
	visible map[string]*recordType
	all     map[string]*recordType
}

func collectRecords(modules map[string]*module) map[string]*recordType {
	all := make(map[string]*recordType)
	for _, module := range modules {
		for _, record := range module.recordOrder {
			all[record.id()] = record
		}
	}
	return all
}

func (c *checker) importedRecords(uses []string) map[string]*recordType {
	records := make(map[string]*recordType)
	for _, name := range uses {
		if module := c.modules[name]; module != nil {
			for _, record := range module.recordOrder {
				if !strings.HasPrefix(record.name, "_") {
					records[record.id()] = record
				}
			}
		}
	}
	return records
}

func (c *checker) privateRecord(name string) (*recordType, *module) {
	prefix, local, ok := strings.Cut(name, ".")
	if !ok || !strings.HasPrefix(local, "_") || !c.used(prefix) {
		return nil, nil
	}
	module := c.modules[prefix]
	if module != nil {
		for _, record := range module.recordOrder {
			if record.name == local {
				return record, module
			}
		}
	}
	return nil, nil
}

func (c *checker) typeError(pos position, typ valueType) error {
	var findPrivate func(valueType) (*recordType, *module)
	findPrivate = func(t valueType) (*recordType, *module) {
		base, args, ok := typeApplication(t)
		if ok {
			if record, module := c.privateRecord(base); record != nil {
				return record, module
			}
			for _, arg := range args {
				if record, module := findPrivate(arg); record != nil {
					return record, module
				}
			}
			return nil, nil
		}
		return c.privateRecord(string(t))
	}
	if record, module := findPrivate(typ); record != nil {
		return withHint(errorAt(pos, CodePrivateUse, "%q is private to module %q", record.name, module.name), fmt.Sprintf("remove the leading underscore in %s to export it", module.path))
	}
	return errorAt(pos, CodeUnknownType, "unknown type %q", typ)
}

func (c *checker) normalizeType(typ valueType) (valueType, bool) {
	return c.normalizeTypeSeen(typ, map[*recordType]bool{})
}
func (c *checker) normalizeTypeSeen(typ valueType, seen map[*recordType]bool) (valueType, bool) {
	if base, args, ok := typeApplication(typ); ok && base == "Map" {
		if len(args) != 2 {
			return "", false
		}
		key, kok := c.normalizeTypeSeen(args[0], seen)
		val, vok := c.normalizeTypeSeen(args[1], seen)
		if !kok || !vok || (key != stringType && key != intType && key != boolType) || containsResultType(val) {
			return "", false
		}
		return valueType("Map<" + string(key) + ", " + string(val) + ">"), true
	}
	if base, args, ok := typeApplication(typ); ok && base != "Result" && base != "List" && base != "Array" {
		if record := c.types.visible[base]; record != nil && record.typeParam != "" && len(args) == 1 {
			inner, ok := c.normalizeTypeSeen(args[0], seen)
			if !ok || isResult(inner) {
				return "", false
			}
			return valueType(record.id() + "<" + string(inner) + ">"), true
		}
		return "", false
	}
	if kind, inner, ok := collectionInner(typ); ok {
		normalized, found := c.normalizeTypeSeen(inner, seen)
		if !found || isResult(normalized) {
			return "", false
		}
		return valueType(kind + "<" + string(normalized) + ">"), true
	}
	if inner, ok := resultInner(typ); ok {
		switch resultFamily(typ) {
		case "InputError", "ParseError", "FileError", "BufferError", "Error":
		default:
			return "", false
		}
		normalized, found := c.normalizeTypeSeen(inner, seen)
		if !found || containsResultType(normalized) {
			return "", false
		}
		return valueType("Result<" + string(normalized) + ", " + resultFamily(typ) + ">"), true
	}
	switch typ {
	case intType, boolType, stringType, unitType, bufferType, fileType, errorType, dateType, timeType, durationType, instantType, timezoneType, zonedType:
		return typ, true
	}
	if record := c.types.visible[string(typ)]; record != nil && record.typeParam == "" {
		if record.alias {
			if record.aliasResolved {
				return record.aliasTarget, true
			}
			if seen[record] || len(seen) >= 256 {
				return "", false
			}
			seen[record] = true
			target, ok := c.normalizeTypeSeen(record.aliasTarget, seen)
			delete(seen, record)
			if !ok {
				return "", false
			}
			record.aliasTarget, record.aliasResolved = target, true
			return target, true
		}
		return valueType(record.id()), true
	}
	return "", false
}
func (c *checker) validType(typ valueType) bool { _, ok := c.normalizeType(typ); return ok }

func privateTypeInModule(typ valueType, module string) bool {
	if base, args, ok := typeApplication(typ); ok {
		if module != "" && strings.HasPrefix(base, module+"._") {
			return true
		}
		for _, arg := range args {
			if privateTypeInModule(arg, module) {
				return true
			}
		}
		return false
	}
	return module != "" && strings.HasPrefix(string(typ), module+"._")
}
func privateTypeLeak(pos position, typ valueType) error {
	return withHint(errorAt(pos, CodePrivateUse, "public declaration exposes private type %s", typ), "make this declaration private or use a public type")
}

func (c *checker) registerRecords(records []*recordType) error {
	if c.types.visible == nil {
		c.types.visible = make(map[string]*recordType)
	}
	if c.types.all == nil {
		c.types.all = make(map[string]*recordType)
	}
	c.types.visible["Option"] = builtinOption
	c.types.all["Option"] = builtinOption
	if c.used("Option") {
		if module := c.modules["Option"]; module != nil {
			for _, variant := range builtinOption.variants {
				if module.words[variant.name] != nil {
					return errorAt(module.words[variant.name].pos, CodeDuplicateName, "imported word conflicts with Option.%s", variant.name)
				}
				for _, record := range module.recordOrder {
					if record.name == variant.name {
						return errorAt(record.pos, CodeDuplicateName, "imported type conflicts with Option.%s", variant.name)
					}
				}
			}
		}
	}
	for _, record := range records {
		if record.name == "Map" || record.name == "Option" || record.name == "List" || record.name == "Array" || record.name == "Result" || record.name == "InputError" || record.name == "ParseError" || record.name == "FileError" || record.name == "BufferError" || strings.Contains(record.name, ".") || c.types.visible[record.name] != nil || c.words[record.name] != nil || c.validType(valueType(record.name)) || HostWord(record.name) {
			return errorAt(record.pos, CodeDuplicateName, "type %q is invalid or already defined", record.name)
		}
		c.types.visible[record.name], c.types.visible[record.id()], c.types.all[record.id()] = record, record, record
	}
	if err := c.validateAliases(records); err != nil {
		return err
	}
	for _, record := range records {
		if record.alias {
			if record.module != "" && !strings.HasPrefix(record.name, "_") && c.rawPrivateType(record.aliasSource, record.module) {
				return privateTypeLeak(record.pos, record.aliasSource)
			}
			if _, ok := c.normalizeType(valueType(record.id())); !ok {
				return errorAt(record.pos, CodeUnknownType, "alias %q has an unknown or recursive target", record.name)
			}
		}
	}
	for _, record := range records {
		if record.typeParam != "" && (record.typeParam == "_" || strings.Contains(record.typeParam, ".") || record.typeParam == record.name || c.types.visible[record.typeParam] != nil || c.validType(valueType(record.typeParam)) || record.typeParam == "Map" || record.typeParam == "List" || record.typeParam == "Array" || record.typeParam == "Result" || record.typeParam == "InputError" || record.typeParam == "ParseError" || record.typeParam == "FileError" || record.typeParam == "BufferError") {
			return errorAt(record.pos, CodeDuplicateName, "invalid type parameter %q", record.typeParam)
		}
		variants := make(map[string]bool)
		for i := range record.variants {
			variant := &record.variants[i]
			if strings.Contains(variant.name, ".") || variants[variant.name] {
				return errorAt(variant.pos, CodeDuplicateName, "variant %q is invalid or already defined", variant.name)
			}
			variants[variant.name] = true
			if c.used(record.name) && !strings.HasPrefix(variant.name, "_") {
				if module := c.modules[record.name]; module != nil {
					if module.words[variant.name] != nil {
						return errorAt(variant.pos, CodeDuplicateName, "variant %q conflicts with an imported word", variant.name)
					}
					for _, imported := range module.recordOrder {
						if imported.name == variant.name {
							return errorAt(variant.pos, CodeDuplicateName, "variant %q conflicts with an imported type", variant.name)
						}
					}
				}
			}
			if c.words[record.name+"."+variant.name] != nil || HostWord(record.name+"."+variant.name) {
				return errorAt(variant.pos, CodeDuplicateName, "variant %q conflicts with a word", variant.name)
			}
			names := make(map[string]bool)
			for j := range variant.fields {
				field := &variant.fields[j]
				if strings.Contains(field.name, ".") || names[field.name] {
					return errorAt(field.pos, CodeDuplicateName, "payload %q is invalid or already defined", field.name)
				}
				names[field.name] = true
				if record.module != "" && !strings.HasPrefix(record.name, "_") && c.rawPrivateType(field.typ, record.module) {
					return privateTypeLeak(field.pos, field.typ)
				}
				typ, ok := c.normalizeTemplate(field.typ, record.typeParam)
				if !ok || isResult(typ) {
					return c.typeError(field.pos, field.typ)
				}
				field.typ = typ
				if record.module != "" && !strings.HasPrefix(record.name, "_") && privateTypeInModule(typ, record.module) {
					return privateTypeLeak(field.pos, typ)
				}
			}
		}
		fields := make(map[string]bool)
		for i := range record.fields {
			field := &record.fields[i]
			if strings.Contains(field.name, ".") || fields[field.name] {
				return errorAt(field.pos, CodeDuplicateName, "field %q is invalid or already defined", field.name)
			}
			fields[field.name] = true
			if record.module != "" && !strings.HasPrefix(record.name, "_") && c.rawPrivateType(field.typ, record.module) {
				return privateTypeLeak(field.pos, field.typ)
			}
			typ, ok := c.normalizeType(field.typ)
			if !ok {
				return c.typeError(field.pos, field.typ)
			}
			if containsResultType(typ) {
				return errorAt(field.pos, CodeUnhandledResult, "a Result cannot be stored in a record")
			}
			field.typ = typ
			if record.module != "" && !strings.HasPrefix(record.name, "_") && privateTypeInModule(typ, record.module) {
				return privateTypeLeak(field.pos, typ)
			}
		}
	}
	active, done := map[string]bool{}, map[string]int{}
	var visit func(*recordType, int) error
	visit = func(record *recordType, depth int) error {
		if depth > 256 {
			return errorAt(record.pos, CodeNestingLimit, "type nesting limit exceeded")
		}
		if active[record.id()] {
			return errorAt(record.pos, CodeUnknownType, "recursive named type %q is not supported", record.id())
		}
		if height := done[record.id()]; height > 0 {
			if depth+height-1 > 256 {
				return errorAt(record.pos, CodeNestingLimit, "type nesting limit exceeded")
			}
			return nil
		}
		active[record.id()] = true
		height := 1
		fields := append([]parameter(nil), record.fields...)
		for _, variant := range record.variants {
			fields = append(fields, variant.fields...)
		}
		var visitType func(valueType) error
		visitType = func(typ valueType) error {
			base, args, applied := typeApplication(typ)
			name := string(typ)
			if applied {
				name = base
			}
			if dep := c.types.all[name]; dep != nil {
				if err := visit(dep, depth+1); err != nil {
					return err
				}
				if candidate := 1 + done[dep.id()]; candidate > height {
					height = candidate
				}
			}
			if applied {
				for _, arg := range args {
					if err := visitType(arg); err != nil {
						return err
					}
				}
			}
			return nil
		}
		for _, field := range fields {
			if err := visitType(field.typ); err != nil {
				return err
			}
		}
		delete(active, record.id())
		done[record.id()] = height
		return nil
	}
	for _, record := range records {
		if err := visit(record, 1); err != nil {
			return err
		}
	}
	return nil
}

// resolveVariant finds a visible nominal choice constructor by its full name.
func (c *checker) resolveVariant(name string) (*recordType, int) {
	i := strings.LastIndex(name, ".")
	if i < 0 {
		return nil, -1
	}
	choice := c.types.visible[name[:i]]
	if choice == nil || !choice.choice {
		return nil, -1
	}
	for n, variant := range choice.variants {
		if variant.name == name[i+1:] {
			return choice, n
		}
	}
	return nil, -1
}

// privateVariant identifies an inaccessible choice without exposing it to lookup.
func (c *checker) privateVariant(name string) (*recordType, *module) {
	i := strings.LastIndex(name, ".")
	if i < 0 {
		return nil, nil
	}
	choice, module := c.privateRecord(name[:i])
	if choice == nil || !choice.choice {
		return nil, nil
	}
	for _, variant := range choice.variants {
		if variant.name == name[i+1:] {
			return choice, module
		}
	}
	return nil, nil
}

// typeApplication splits nested type arguments without confusing inner commas.
func typeApplication(typ valueType) (string, []valueType, bool) {
	text := string(typ)
	i := strings.IndexByte(text, '<')
	if i <= 0 || !strings.HasSuffix(text, ">") {
		return "", nil, false
	}
	body := text[i+1 : len(text)-1]
	depth, start := 0, 0
	var args []valueType
	for at, r := range body {
		switch r {
		case '<':
			depth++
		case '>':
			depth--
			if depth < 0 {
				return "", nil, false
			}
		case ',':
			if depth == 0 {
				args = append(args, valueType(strings.TrimSpace(body[start:at])))
				start = at + 1
			}
		}
	}
	if depth != 0 {
		return "", nil, false
	}
	args = append(args, valueType(strings.TrimSpace(body[start:])))
	for _, arg := range args {
		if arg == "" {
			return "", nil, false
		}
	}
	return text[:i], args, true
}
func substituteType(typ valueType, param string, arg valueType) valueType {
	if string(typ) == param {
		return arg
	}
	base, args, ok := typeApplication(typ)
	if !ok {
		return typ
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = string(substituteType(a, param, arg))
	}
	return valueType(base + "<" + strings.Join(parts, ", ") + ">")
}
func inferTypeArgument(pattern, actual valueType, param string, found *valueType) bool {
	if string(pattern) == param {
		if *found == "" {
			*found = actual
			return true
		}
		return *found == actual
	}
	pbase, pargs, pok := typeApplication(pattern)
	abase, aargs, aok := typeApplication(actual)
	if !pok || !aok {
		return pattern == actual
	}
	if pbase != abase || len(pargs) != len(aargs) {
		return false
	}
	for i := range pargs {
		if !inferTypeArgument(pargs[i], aargs[i], param, found) {
			return false
		}
	}
	return true
}
func (c *checker) normalizeTemplate(typ valueType, param string) (valueType, bool) {
	if string(typ) == param && param != "" {
		return typ, true
	}
	base, args, ok := typeApplication(typ)
	if !ok {
		return c.normalizeType(typ)
	}
	normalized := make([]string, len(args))
	for i, arg := range args {
		v, ok := c.normalizeTemplate(arg, param)
		if !ok || isResult(v) {
			return "", false
		}
		normalized[i] = string(v)
	}
	if (base == "List" || base == "Array") && len(args) == 1 {
		return valueType(base + "<" + normalized[0] + ">"), true
	}
	if base == "Map" && len(args) == 2 && (normalized[0] == string(stringType) || normalized[0] == string(intType) || normalized[0] == string(boolType)) {
		return valueType("Map<" + normalized[0] + ", " + normalized[1] + ">"), true
	}
	if record := c.types.visible[base]; record != nil && record.typeParam != "" && len(args) == 1 {
		return valueType(record.id() + "<" + normalized[0] + ">"), true
	}
	return "", false
}

func typeContainsParam(typ valueType, param string) bool {
	if string(typ) == param {
		return true
	}
	_, args, ok := typeApplication(typ)
	if !ok {
		return false
	}
	for _, arg := range args {
		if typeContainsParam(arg, param) {
			return true
		}
	}
	return false
}
func choiceTypeArg(choice *recordType, typ valueType) valueType {
	base, args, ok := typeApplication(typ)
	if ok && base == choice.id() && len(args) == 1 {
		return args[0]
	}
	return ""
}

func containsResultType(typ valueType) bool {
	if isResult(typ) {
		return true
	}
	_, args, ok := typeApplication(typ)
	if !ok {
		return false
	}
	for _, arg := range args {
		if containsResultType(arg) {
			return true
		}
	}
	return false
}

func mapTypes(typ valueType) (valueType, valueType, bool) {
	base, args, ok := typeApplication(typ)
	if !ok || base != "Map" || len(args) != 2 {
		return "", "", false
	}
	return args[0], args[1], true
}

// rawPrivateType catches private aliases before transparent normalization erases their names.
func (c *checker) rawPrivateType(typ valueType, module string) bool {
	base, args, applied := typeApplication(typ)
	name := string(typ)
	if applied {
		name = base
	}
	if record := c.types.visible[name]; record != nil && record.module == module && strings.HasPrefix(record.name, "_") {
		return true
	}
	if applied {
		for _, arg := range args {
			if c.rawPrivateType(arg, module) {
				return true
			}
		}
	}
	return false
}

func (c *checker) validateAliases(records []*recordType) error {
	active := map[*recordType]bool{}
	done := map[*recordType]int{}
	var visit func(*recordType, int) error
	visit = func(alias *recordType, depth int) error {
		if depth > 256 {
			return errorAt(alias.pos, CodeNestingLimit, "type nesting limit exceeded")
		}
		if active[alias] {
			return errorAt(alias.pos, CodeUnknownType, "recursive alias %q is not supported", alias.name)
		}
		if height := done[alias]; height > 0 {
			if depth+height-1 > 256 {
				return errorAt(alias.pos, CodeNestingLimit, "type nesting limit exceeded")
			}
			return nil
		}
		active[alias] = true
		height := 1
		var walk func(valueType, int) error
		walk = func(t valueType, nesting int) error {
			if depth+nesting > 256 {
				return errorAt(alias.pos, CodeNestingLimit, "type nesting limit exceeded")
			}
			base, args, applied := typeApplication(t)
			name := string(t)
			if applied {
				name = base
			}
			if dep := c.types.visible[name]; dep != nil && dep.alias {
				if err := visit(dep, depth+nesting+1); err != nil {
					return err
				}
				if candidate := nesting + 1 + done[dep]; candidate > height {
					height = candidate
				}
			}
			if applied {
				if nesting+2 > height {
					height = nesting + 2
				}
				for _, arg := range args {
					if err := walk(arg, nesting+1); err != nil {
						return err
					}
				}
			}
			return nil
		}
		target := alias.aliasSource
		// Imported aliases have already been resolved in their defining module;
		// their original names need not be visible from this module.
		if alias.aliasResolved {
			target = alias.aliasTarget
		}
		if err := walk(target, 0); err != nil {
			return err
		}
		delete(active, alias)
		done[alias] = height
		return nil
	}
	for _, record := range records {
		if record.alias {
			if err := visit(record, 1); err != nil {
				return err
			}
		}
	}
	return nil
}

// Record defaults execute in the declaration's module and cannot see caller bindings.
func (c *checker) checkRecordDefaults(records []*recordType) error {
	for _, record := range records {
		for _, field := range record.fields {
			if field.defaultExpr == nil {
				continue
			}
			effects := map[string]bool{}
			c.effectExpr(field.defaultExpr, map[string]string{}, nil, effects)
			if len(effects) != 0 {
				return errorAt(field.pos, CodeTryInput, "record default cannot propagate a Result")
			}
			typ, err := c.expressionExpected(field.defaultExpr, newScope(nil), field.typ)
			if err != nil {
				return err
			}
			if typ != field.typ && typ != neverType {
				return errorAt(field.pos, CodeArgumentType, "default for %s requires %s, found %s", field.name, field.typ, typ)
			}
		}
	}
	return nil
}
