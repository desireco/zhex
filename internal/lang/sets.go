package lang

// setValue is Zhex Set<T>: insertion-ordered members unique by value, keyed
// through the map-key machinery (the map-key domain is the set-element
// domain). Active guards traversal-shaped mutation words the way maps do.
type setValue struct {
	elemType valueType
	entries  []mapKey
	index    map[mapKey]int
	active   int
}

func (s *setValue) key(v value) mapKey {
	switch s.elemType {
	case stringType:
		return mapKey{text: v.text}
	case intType:
		return mapKey{number: v.number}
	default:
		return mapKey{boolean: v.boolean}
	}
}

func (s *setValue) keyValue(k mapKey) value {
	switch s.elemType {
	case stringType:
		return value{text: k.text}
	case intType:
		return value{number: k.number}
	default:
		return value{boolean: k.boolean}
	}
}

func (s *setValue) has(v value) bool {
	_, ok := s.index[s.key(v)]
	return ok
}

// add inserts a member; an existing member is a no-op.
func (s *setValue) add(v value) {
	k := s.key(v)
	if _, ok := s.index[k]; ok {
		return
	}
	s.index[k] = len(s.entries)
	s.entries = append(s.entries, k)
}

// remove deletes a member, preserving order; an absent member is a no-op.
func (s *setValue) remove(v value) {
	k := s.key(v)
	i, ok := s.index[k]
	if !ok {
		return
	}
	delete(s.index, k)
	copy(s.entries[i:], s.entries[i+1:])
	s.entries[len(s.entries)-1] = mapKey{}
	s.entries = s.entries[:len(s.entries)-1]
	for n := i; n < len(s.entries); n++ {
		s.index[s.entries[n]] = n
	}
}

func (s *setValue) guard(pos position, operation string) error {
	if s.active == 0 {
		return nil
	}
	d := errorAt(pos, CodeCollectionIterating, "collection is being iterated").(*diagnostic)
	d.operation = operation
	return d
}
