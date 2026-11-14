package lang

type mapKey struct {
	text    string
	number  int64
	boolean bool
}
type mapEntry struct {
	key   mapKey
	value value
}
type mapValue struct {
	keyType valueType
	entries []mapEntry
	index   map[mapKey]int
	active  int
}

func (m *mapValue) key(v value) mapKey {
	switch m.keyType {
	case stringType:
		return mapKey{text: v.text}
	case intType:
		return mapKey{number: v.number}
	default:
		return mapKey{boolean: v.boolean}
	}
}
func (m *mapValue) put(key, item value) {
	k := m.key(key)
	if i, ok := m.index[k]; ok {
		m.entries[i].value = item
		return
	}
	m.index[k] = len(m.entries)
	m.entries = append(m.entries, mapEntry{key: k, value: item})
}
func (m *mapValue) lookup(key value) value {
	if i, ok := m.index[m.key(key)]; ok {
		return value{choice: &choiceValue{name: "Option.Some", variant: 1, fields: []value{m.entries[i].value}}}
	}
	return value{choice: &choiceValue{name: "Option.None", variant: 0}}
}
func (m *mapValue) guard(pos position, operation string) error {
	if m.active == 0 {
		return nil
	}
	d := errorAt(pos, CodeCollectionIterating, "collection is being iterated").(*diagnostic)
	d.operation = operation
	return d
}
func (m *mapValue) remove(key value) {
	k := m.key(key)
	i, ok := m.index[k]
	if !ok {
		return
	}
	delete(m.index, k)
	copy(m.entries[i:], m.entries[i+1:])
	m.entries[len(m.entries)-1] = mapEntry{}
	m.entries = m.entries[:len(m.entries)-1]
	for n := i; n < len(m.entries); n++ {
		m.index[m.entries[n].key] = n
	}
}
func (m *mapValue) merge(source *mapValue) {
	if m == source {
		return
	}
	for _, entry := range source.entries {
		if i, ok := m.index[entry.key]; ok {
			m.entries[i].value = entry.value
		} else {
			m.index[entry.key] = len(m.entries)
			m.entries = append(m.entries, entry)
		}
	}
}

func (m *mapValue) keyValue(k mapKey) value {
	switch m.keyType {
	case stringType:
		return value{text: k.text}
	case intType:
		return value{number: k.number}
	default:
		return value{boolean: k.boolean}
	}
}
