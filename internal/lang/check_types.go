package lang

import (
	"fmt"
	"strings"
)

// namedTypeRegistry separates the declarations visible in the current file
// from the full graph needed to check imported field types and cycles.
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
	for {
		if inner, ok := resultInner(typ); ok {
			typ = inner
			continue
		}
		if _, inner, ok := collectionInner(typ); ok {
			typ = inner
			continue
		}
		break
	}
	if record, module := c.privateRecord(string(typ)); record != nil {
		return withHint(errorAt(pos, CodePrivateUse, "%q is private to module %q", record.name, module.name), fmt.Sprintf("remove the leading underscore in %s to export it", module.path))
	}
	return errorAt(pos, CodeUnknownType, "unknown type %q", typ)
}

func (c *checker) normalizeType(typ valueType) (valueType, bool) {
	if kind, inner, ok := collectionInner(typ); ok {
		normalized, found := c.normalizeType(inner)
		if !found || isResult(normalized) {
			return "", false
		}
		return valueType(kind + "<" + string(normalized) + ">"), true
	}
	if inner, ok := resultInner(typ); ok {
		normalized, found := c.normalizeType(inner)
		if !found {
			return "", false
		}
		return valueType("Result<" + string(normalized) + ", " + resultFamily(typ) + ">"), true
	}
	switch typ {
	case intType, boolType, stringType, unitType, bufferType, fileType, errorType:
		return typ, true
	}
	if record := c.types.visible[string(typ)]; record != nil {
		return valueType(record.id()), true
	}
	return "", false
}
func (c *checker) validType(typ valueType) bool { _, ok := c.normalizeType(typ); return ok }

func privateTypeInModule(typ valueType, module string) bool {
	if _, inner, ok := collectionInner(typ); ok {
		return privateTypeInModule(inner, module)
	}
	if inner, ok := resultInner(typ); ok {
		return privateTypeInModule(inner, module)
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
	for _, record := range records {
		if record.name == "List" || record.name == "Array" || record.name == "Result" || record.name == "InputError" || record.name == "ParseError" || record.name == "FileError" || record.name == "BufferError" || strings.Contains(record.name, ".") || c.types.visible[record.name] != nil || c.words[record.name] != nil || c.validType(valueType(record.name)) || HostWord(record.name) {
			return errorAt(record.pos, CodeDuplicateName, "type %q is invalid or already defined", record.name)
		}
		c.types.visible[record.name], c.types.visible[record.id()], c.types.all[record.id()] = record, record, record
	}
	for _, record := range records {
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
			if c.words[record.name+"."+variant.name] != nil {
				return errorAt(variant.pos, CodeDuplicateName, "variant %q conflicts with a word", variant.name)
			}
			names := make(map[string]bool)
			for j := range variant.fields {
				field := &variant.fields[j]
				if strings.Contains(field.name, ".") || names[field.name] {
					return errorAt(field.pos, CodeDuplicateName, "payload %q is invalid or already defined", field.name)
				}
				names[field.name] = true
				typ, ok := c.normalizeType(field.typ)
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
			typ, ok := c.normalizeType(field.typ)
			if !ok {
				return c.typeError(field.pos, field.typ)
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
			return errorAt(record.pos, CodeUnknownType, "recursive record type %q is not supported", record.id())
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
		for _, field := range fields {
			typ := field.typ
			for {
				_, inner, ok := collectionInner(typ)
				if !ok {
					break
				}
				typ = inner
			}
			if dep := c.types.all[string(typ)]; dep != nil {
				if err := visit(dep, depth+1); err != nil {
					return err
				}
				if candidate := 1 + done[dep.id()]; candidate > height {
					height = candidate
				}
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
