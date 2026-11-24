package zhexrt

import (
	"sort"
	"strconv"
	"strings"
)

// MaxElements is the collection element limit the interpreter enforces.
const MaxElements = 1000000

// List is Zhex List<T>: a pointer, so names share one list and Go's GC
// covers constructible cycles. Active counts in-progress traversals so
// mutation words can refuse.
type List[T any] struct {
	Items  []T
	Active int
}

// Option is Zhex Option<T>: Some carries a value, None does not.
type Option[T any] struct {
	Some  bool
	Value T
}

func Some[T any](v T) Option[T] { return Option[T]{Some: true, Value: v} }
func None[T any]() Option[T]    { return Option[T]{} }

// Map is Zhex Map<K, V>: insertion-ordered entries plus an index. Active
// guards traversal.
type Map[K comparable, V any] struct {
	Entries []MapEntry[K, V]
	index   map[K]int
	Active  int
}

type MapEntry[K comparable, V any] struct {
	Key   K
	Value V
}

func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{index: map[K]int{}}
}

func (m *Map[K, V]) put(key K, item V) {
	if i, ok := m.index[key]; ok {
		m.Entries[i].Value = item
		return
	}
	m.index[key] = len(m.Entries)
	m.Entries = append(m.Entries, MapEntry[K, V]{Key: key, Value: item})
}

// MapPut replaces or appends and returns the receiver; MapPutAt is the
// literal-construction helper the emitter calls (no traversal guard, like
// the interpreter's map literal).
func MapPut[K comparable, V any](m *Map[K, V], key K, item V) *Map[K, V] {
	m.put(key, item)
	return m
}

func (m *Map[K, V]) lookup(key K) Option[V] {
	if i, ok := m.index[key]; ok {
		return Some(m.Entries[i].Value)
	}
	return None[V]()
}

// MapRemove deletes by key, preserving order (entries shift and the index
// renumbers), and returns the receiver.
func MapRemove[K comparable, V any](pos Pos, operation string, m *Map[K, V], key K) *Map[K, V] {
	if err := m.guard(pos, operation); err != nil {
		panic(err)
	}
	m.remove(key)
	return m
}

func (m *Map[K, V]) remove(key K) {
	i, ok := m.index[key]
	if !ok {
		return
	}
	delete(m.index, key)
	copy(m.Entries[i:], m.Entries[i+1:])
	m.Entries[len(m.Entries)-1] = MapEntry[K, V]{}
	m.Entries = m.Entries[:len(m.Entries)-1]
	for n := i; n < len(m.Entries); n++ {
		m.index[m.Entries[n].Key] = n
	}
}

func (m *Map[K, V]) guard(pos Pos, operation string) *Error {
	if m.Active == 0 {
		return nil
	}
	return &Error{Pos: pos, Code: "collection-iterating", Operation: operation, Message: "collection is being iterated"}
}

// Set is Zhex Set<T>: insertion-ordered members unique by value (the
// map-key domain). Active guards traversal-shaped mutation, the Map shape.
type Set[T comparable] struct {
	Entries []T
	index   map[T]int
	Active  int
}

func NewSet[T comparable]() *Set[T] { return &Set[T]{index: map[T]int{}} }

func (s *Set[T]) put(item T) {
	if _, ok := s.index[item]; ok {
		return
	}
	s.index[item] = len(s.Entries)
	s.Entries = append(s.Entries, item)
}

func (s *Set[T]) remove(item T) {
	i, ok := s.index[item]
	if !ok {
		return
	}
	delete(s.index, item)
	copy(s.Entries[i:], s.Entries[i+1:])
	s.Entries[len(s.Entries)-1] = item
	s.Entries = s.Entries[:len(s.Entries)-1]
	for n := i; n < len(s.Entries); n++ {
		s.index[s.Entries[n]] = n
	}
}

func (s *Set[T]) guard(pos Pos, operation string) *Error {
	if s.Active == 0 {
		return nil
	}
	return &Error{Pos: pos, Code: "collection-iterating", Operation: operation, Message: "collection is being iterated"}
}

// SetAdd inserts a member (a present one is a no-op) and returns the
// receiver; SetRemove deletes one (an absent one is a no-op) and returns
// the receiver — the map.put/map.remove contract.
func SetAdd[T comparable](pos Pos, operation string, s *Set[T], item T) *Set[T] {
	if err := s.guard(pos, operation); err != nil {
		panic(err)
	}
	s.put(item)
	return s
}

func SetRemove[T comparable](pos Pos, operation string, s *Set[T], item T) *Set[T] {
	if err := s.guard(pos, operation); err != nil {
		panic(err)
	}
	s.remove(item)
	return s
}

func SetContains[T comparable](s *Set[T], item T) bool {
	_, ok := s.index[item]
	return ok
}

func SetLength[T comparable](s *Set[T]) Int { return I64(int64(len(s.Entries))) }

// SetValues extracts a fresh List in insertion order, the MapKeys shape.
func SetValues[T comparable](pos Pos, operation string, s *Set[T]) *List[T] {
	if len(s.Entries) > MaxElements {
		Fail(pos, "collection-limit", "%s exceeds element limit", operation)
	}
	items := make([]T, len(s.Entries))
	copy(items, s.Entries)
	return &List[T]{Items: items}
}

// SetStringInt, SetStringString, and SetStringBool render the print form:
// members in insertion order inside set braces, String members quoted so
// the element type stays visible — the interpreter's renderSet.
func SetStringInt(s *Set[Int]) string {
	return setBraces(s.Entries, func(v Int) string { return Str(v) })
}

func SetStringString(s *Set[string]) string {
	return setBraces(s.Entries, func(v string) string { return strconv.Quote(v) })
}

func SetStringBool(s *Set[bool]) string {
	return setBraces(s.Entries, func(v bool) string { return strconv.FormatBool(v) })
}

func setBraces[T any](entries []T, show func(T) string) string {
	parts := make([]string, len(entries))
	for i, v := range entries {
		parts[i] = show(v)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// MapInto merges source into m, replacing existing values on collisions,
// and returns m.
func MapInto[K comparable, V any](pos Pos, source *Map[K, V], m *Map[K, V]) *Map[K, V] {
	if err := m.guard(pos, "map.into"); err != nil {
		panic(err)
	}
	if m == source {
		return m
	}
	for _, entry := range source.Entries {
		m.put(entry.Key, entry.Value)
	}
	return m
}

// MapLookup is the indexed read m[k]: Some(value) or None.
func MapLookup[K comparable, V any](m *Map[K, V], key K) Option[V] {
	return m.lookup(key)
}

// MapLength, MapContains, MapPutChecked, and MapSetHard are the map
// words; put-shaped mutation refuses under traversal (hard, the plain
// map.put contract).
func MapLength[K comparable, V any](m *Map[K, V]) Int { return I64(int64(len(m.Entries))) }

func MapContains[K comparable, V any](m *Map[K, V], key K) bool {
	_, ok := m.index[key]
	return ok
}

func MapPutChecked[K comparable, V any](pos Pos, operation string, m *Map[K, V], key K, item V) *Map[K, V] {
	if err := m.guard(pos, operation); err != nil {
		panic(err)
	}
	m.put(key, item)
	return m
}

func MapSetHard[K comparable, V any](pos Pos, m *Map[K, V], key K, item V) {
	if err := m.guard(pos, "map.set"); err != nil {
		panic(err)
	}
	m.put(key, item)
}

// MapKeys and MapValues extract fresh lists in insertion order.
func MapKeys[K comparable, V any](pos Pos, operation string, m *Map[K, V]) *List[K] {
	if len(m.Entries) > MaxElements {
		Fail(pos, "collection-limit", "%s exceeds element limit", operation)
	}
	items := make([]K, len(m.Entries))
	for i, entry := range m.Entries {
		items[i] = entry.Key
	}
	return &List[K]{Items: items}
}

func MapValues[K comparable, V any](pos Pos, operation string, m *Map[K, V]) *List[V] {
	if len(m.Entries) > MaxElements {
		Fail(pos, "collection-limit", "%s exceeds element limit", operation)
	}
	items := make([]V, len(m.Entries))
	for i, entry := range m.Entries {
		items[i] = entry.Value
	}
	return &List[V]{Items: items}
}

// ---------------------------------------------------------------- list words

func ListNew[T any]() *List[T] { return &List[T]{} }

// ListAppend is a checked collection operation: the Result selects the
// failure (list-range-free; iteration and limit refuse here).
func ListAppend[T any](pos Pos, l *List[T], item T) Result[*List[T]] {
	if l.Active > 0 {
		return collectionFailure[*List[T]](pos, "collection-iterating", "list.append", "collection is being iterated", nil)
	}
	if len(l.Items) >= MaxElements {
		return collectionFailure[*List[T]](pos, "collection-limit", "list.append", "list exceeds element limit", nil)
	}
	l.Items = append(l.Items, item)
	return Ok(l)
}

func ListInsert[T any](pos Pos, l *List[T], index Int, item T) Result[*List[T]] {
	if l.Active > 0 {
		return collectionFailure[*List[T]](pos, "collection-iterating", "list.insert", "collection is being iterated", nil)
	}
	if len(l.Items) >= MaxElements {
		return collectionFailure[*List[T]](pos, "collection-limit", "list.insert", "list exceeds element limit", nil)
	}
	if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() >= int64(len(l.Items))+1 {
		return collectionFailure[*List[T]](pos, "list-range", "list.insert", "list insert index is out of range", map[string]string{"index": Str(index), "length": strconv.Itoa(len(l.Items))})
	}
	at := index.AsInt64()
	rest := append([]T{item}, l.Items[at:]...)
	l.Items = append(l.Items[:at], rest...)
	return Ok(l)
}

// ListFirst and ListLast read the ends without changing the list; the
// empty failure mirrors the indexed read at 0 / len-1.
func ListFirst[T any](pos Pos, l *List[T]) Result[T] {
	if len(l.Items) == 0 {
		return collectionFailure[T](pos, "list-range", "list.first", "list index is out of range", map[string]string{"index": "0", "length": "0"})
	}
	return Ok(l.Items[0])
}

func ListLast[T any](pos Pos, l *List[T]) Result[T] {
	if len(l.Items) == 0 {
		return collectionFailure[T](pos, "list-range", "list.last", "list index is out of range", map[string]string{"index": "-1", "length": "0"})
	}
	return Ok(l.Items[len(l.Items)-1])
}

// ListGet is the checked indexed read; plain reads harden the failure.
func ListGet[T any](pos Pos, l *List[T], index Int) Result[T] {
	if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() >= int64(len(l.Items)) {
		return collectionFailure[T](pos, "list-range", "list.index", "list index is out of range", map[string]string{"index": Str(index), "length": strconv.Itoa(len(l.Items))})
	}
	return Ok(l.Items[index.AsInt64()])
}

func ListSet[T any](pos Pos, l *List[T], index Int, item T) Result[Unit] {
	if l.Active > 0 {
		return collectionFailure[Unit](pos, "collection-iterating", "list.set", "collection is being iterated", nil)
	}
	if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() >= int64(len(l.Items)) {
		return collectionFailure[Unit](pos, "list-range", "list.set", "list index is out of range", map[string]string{"index": Str(index), "length": strconv.Itoa(len(l.Items))})
	}
	l.Items[index.AsInt64()] = item
	return Ok(Unit{})
}

func ListLength[T any](l *List[T]) Int { return I64(int64(len(l.Items))) }

// The higher-order words take Go funcs (partials and word values close
// over generated code); callbacks cannot fail — the fallible-callback
// form fences at emission.

func ListMap[T, U any](l *List[T], f func(T) U) *List[U] {
	l.Active++
	defer func() { l.Active-- }()
	items := make([]U, 0, len(l.Items))
	for _, item := range l.Items {
		items = append(items, f(item))
	}
	return &List[U]{Items: items}
}

func ListFilter[T any](l *List[T], f func(T) bool) *List[T] {
	l.Active++
	defer func() { l.Active-- }()
	items := make([]T, 0, len(l.Items))
	for _, item := range l.Items {
		if f(item) {
			items = append(items, item)
		}
	}
	return &List[T]{Items: items}
}

func ListFold[T, U any](l *List[T], accumulator U, f func(U, T) U) U {
	l.Active++
	defer func() { l.Active-- }()
	for _, item := range l.Items {
		accumulator = f(accumulator, item)
	}
	return accumulator
}

func ListFind[T any](l *List[T], f func(T) bool) Option[T] {
	l.Active++
	defer func() { l.Active-- }()
	for _, item := range l.Items {
		if f(item) {
			return Some(item)
		}
	}
	return None[T]()
}

func ListAny[T any](l *List[T], f func(T) bool) bool {
	l.Active++
	defer func() { l.Active-- }()
	for _, item := range l.Items {
		if f(item) {
			return true
		}
	}
	return false
}

func ListAll[T any](l *List[T], f func(T) bool) bool {
	l.Active++
	defer func() { l.Active-- }()
	for _, item := range l.Items {
		if !f(item) {
			return false
		}
	}
	return true
}

func ListContains[T comparable](l *List[T], item T) bool {
	l.Active++
	defer func() { l.Active-- }()
	for _, entry := range l.Items {
		if entry == item {
			return true
		}
	}
	return false
}

// ListSortInts and ListSortStrings are sort's constrained overloads:
// fresh ascending copies, source unchanged, no traversal guard (the
// source is only read). Equal keys keep source order.

func ListSortInts(l *List[Int]) *List[Int] {
	sorted := append([]Int(nil), l.Items...)
	sort.SliceStable(sorted, func(i, j int) bool { return Cmp(sorted[i], sorted[j]) < 0 })
	return &List[Int]{Items: sorted}
}

func ListSortStrings(l *List[string]) *List[string] {
	sorted := append([]string(nil), l.Items...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return &List[string]{Items: sorted}
}

// ListSortByInt and ListSortByString rank any list by a key evaluated
// once per element in source order; equal keys keep source order.
func ListSortByInt[T any](l *List[T], key func(T) Int) *List[T] {
	l.Active++
	defer func() { l.Active-- }()
	keys := make([]Int, len(l.Items))
	for i, item := range l.Items {
		keys[i] = key(item)
	}
	return sortBy(l.Items, keys, func(a, b Int) bool { return Cmp(a, b) < 0 })
}

func ListSortByString[T any](l *List[T], key func(T) string) *List[T] {
	l.Active++
	defer func() { l.Active-- }()
	keys := make([]string, len(l.Items))
	for i, item := range l.Items {
		keys[i] = key(item)
	}
	return sortBy(l.Items, keys, func(a, b string) bool { return a < b })
}

func sortBy[T, K any](items []T, keys []K, less func(a, b K) bool) *List[T] {
	order := make([]int, len(items))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return less(keys[order[a]], keys[order[b]]) })
	sorted := make([]T, len(items))
	for i, at := range order {
		sorted[i] = items[at]
	}
	return &List[T]{Items: sorted}
}

// ---------------------------------------------------------------- array words

// Array is Zhex Array<T>: a shape plus row-major items, shared by pointer.
type Array[T any] struct {
	Shape  []int64
	Items  []T
	Active int
}

func collectionFailure[T any](pos Pos, code, operation, message string, details map[string]string) Result[T] {
	return Result[T]{Err: &Error{Pos: pos, Code: code, Operation: operation, Message: message, Details: details}}
}

// ArrayNew is checked (array.new): rank 1..32, nonnegative extents, the
// product within the element limit.
func ArrayNew[T any](pos Pos, dims *List[Int], fill T) Result[*Array[T]] {
	shape, count, failure := arrayShape[T](pos, dims, "array.new")
	if failure != nil {
		return Result[*Array[T]]{Err: failure}
	}
	items := make([]T, count)
	for i := range items {
		items[i] = fill
	}
	return Ok(&Array[T]{Shape: shape, Items: items})
}

func arrayShape[T any](pos Pos, dims *List[Int], operation string) ([]int64, int, *Error) {
	if len(dims.Items) < 1 || len(dims.Items) > 32 {
		return nil, 0, &Error{Pos: pos, Code: "array-rank", Operation: operation, Message: "array rank must be between 1 and 32", Details: map[string]string{"rank": strconv.Itoa(len(dims.Items)), "min_rank": "1", "max_rank": "32"}}
	}
	shape := make([]int64, len(dims.Items))
	zero := false
	for i, v := range dims.Items {
		if v.Negative() {
			return nil, 0, &Error{Pos: pos, Code: "array-shape", Operation: operation, Message: "array extents cannot be negative", Details: map[string]string{"axis": strconv.Itoa(i), "extent": Str(v)}}
		}
		if !v.Fits64() {
			return nil, 0, &Error{Pos: pos, Code: "collection-limit", Operation: operation, Message: "array exceeds element limit", Details: map[string]string{"limit": strconv.Itoa(MaxElements)}}
		}
		shape[i] = v.AsInt64()
		zero = zero || v.IsZero()
	}
	if zero {
		return shape, 0, nil
	}
	count := int64(1)
	for _, n := range shape {
		if n > MaxElements/count {
			return nil, 0, &Error{Pos: pos, Code: "collection-limit", Operation: operation, Message: "array exceeds element limit", Details: map[string]string{"limit": strconv.Itoa(MaxElements)}}
		}
		count *= n
	}
	return shape, int(count), nil
}

// ArraySet is the checked indexed write; it refuses under traversal.
func ArraySet[T any](pos Pos, a *Array[T], indices []Int, item T) Result[Unit] {
	const operation = "array.set"
	if a.Active > 0 {
		return collectionFailure[Unit](pos, "collection-iterating", operation, "collection is being iterated", nil)
	}
	if len(indices) != len(a.Shape) {
		return collectionFailure[Unit](pos, "array-rank", operation, "array index has the wrong rank", map[string]string{"rank": strconv.Itoa(len(indices)), "expected_rank": strconv.Itoa(len(a.Shape))})
	}
	offset := int64(0)
	for i, index := range indices {
		if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() >= a.Shape[i] {
			return collectionFailure[Unit](pos, "array-range", operation, "array coordinate is out of range", map[string]string{"axis": strconv.Itoa(i), "index": Str(index), "extent": strconv.FormatInt(a.Shape[i], 10)})
		}
		offset = offset*a.Shape[i] + index.AsInt64()
	}
	a.Items[offset] = item
	return Ok(Unit{})
}

// ArrayResize keeps in-bounds elements at their old coordinates.
func ArrayResize[T any](pos Pos, a *Array[T], dims *List[Int], fill T) Result[*Array[T]] {
	if a.Active > 0 {
		return collectionFailure[*Array[T]](pos, "collection-iterating", "array.resize", "collection is being iterated", nil)
	}
	shape, count, failure := arrayShape[T](pos, dims, "array.resize")
	if failure != nil {
		return Result[*Array[T]]{Err: failure}
	}
	if len(shape) != len(a.Shape) {
		return collectionFailure[*Array[T]](pos, "array-rank", "array.resize", "array resize cannot change rank", map[string]string{"rank": strconv.Itoa(len(shape)), "expected_rank": strconv.Itoa(len(a.Shape))})
	}
	items := make([]T, count)
	for i := range items {
		items[i] = fill
	}
	for old, v := range a.Items {
		index := int64(old)
		offset := int64(0)
		stride := int64(1)
		fits := true
		for axis := len(shape) - 1; axis >= 0; axis-- {
			coord := index % a.Shape[axis]
			index /= a.Shape[axis]
			if coord >= shape[axis] {
				fits = false
				break
			}
			offset += coord * stride
			stride *= shape[axis]
		}
		if fits {
			items[offset] = v
		}
	}
	a.Shape, a.Items = shape, items
	return Ok(a)
}

// ArrayGet reads one element (the emitter arranges the Result's value;
// writes and reads share ArrayIndex's checks).
func ArrayGet[T any](pos Pos, a *Array[T], indices []Int) Result[T] {
	operation := "array.index"
	if len(indices) != len(a.Shape) {
		return collectionFailure[T](pos, "array-rank", operation, "array index has the wrong rank", map[string]string{"rank": strconv.Itoa(len(indices)), "expected_rank": strconv.Itoa(len(a.Shape))})
	}
	offset := int64(0)
	for i, index := range indices {
		if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() >= a.Shape[i] {
			return collectionFailure[T](pos, "array-range", operation, "array coordinate is out of range", map[string]string{"axis": strconv.Itoa(i), "index": Str(index), "extent": strconv.FormatInt(a.Shape[i], 10)})
		}
		offset = offset*a.Shape[i] + index.AsInt64()
	}
	return Ok(a.Items[offset])
}

// ArraySelect keeps whole rank-2 rows passing the predicate; each row is
// a fresh list sharing the elements, the rectangle preserved.
func ArraySelect[T any](pos Pos, a *Array[T], f func(*List[T]) bool) *Array[T] {
	if len(a.Shape) != 2 {
		Fail(pos, "array-rank", "array.select requires a rank-2 array, found rank %d", len(a.Shape))
	}
	a.Active++
	defer func() { a.Active-- }()
	width := a.Shape[1]
	kept := make([]T, 0, len(a.Items))
	rows := 0
	for row := int64(0); row < a.Shape[0]; row++ {
		items := append([]T(nil), a.Items[row*width:(row+1)*width]...)
		if f(&List[T]{Items: items}) {
			kept = append(kept, items...)
			rows++
		}
	}
	return &Array[T]{Shape: []int64{int64(rows), width}, Items: kept}
}

func ArrayLength[T any](a *Array[T]) Int { return I64(int64(len(a.Items))) }
func ArrayRank[T any](a *Array[T]) Int   { return I64(int64(len(a.Shape))) }

func ArrayShape[T any](a *Array[T]) *List[Int] {
	items := make([]Int, len(a.Shape))
	for i, n := range a.Shape {
		items[i] = I64(n)
	}
	return &List[Int]{Items: items}
}

// ---------------------------------------------------------------- buffer

// Buffer is Zhex Buffer: lines behind a pointer, so names alias one
// buffer.
type Buffer struct{ Lines []string }

func BufferNew() *Buffer { return &Buffer{} }

func BufferAddLine(b *Buffer, line string) *Buffer {
	b.Lines = append(b.Lines, line)
	return b
}

func BufferLineCount(b *Buffer) Int { return I64(int64(len(b.Lines))) }

func BufferLine(pos Pos, b *Buffer, index Int) Result[string] {
	if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() >= int64(len(b.Lines)) {
		return collectionFailure[string](pos, "buffer-range", "buffer.line", "buffer index "+Str(index)+" is outside 0.."+strconv.Itoa(len(b.Lines)-1), map[string]string{"index": Str(index), "line-count": strconv.Itoa(len(b.Lines))})
	}
	return Ok(b.Lines[index.AsInt64()])
}

func BufferSetLine(pos Pos, b *Buffer, index Int, line string) Result[*Buffer] {
	if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() >= int64(len(b.Lines)) {
		return collectionFailure[*Buffer](pos, "buffer-range", "buffer.set-line", "buffer index "+Str(index)+" is outside 0.."+strconv.Itoa(len(b.Lines)-1), map[string]string{"index": Str(index), "line-count": strconv.Itoa(len(b.Lines))})
	}
	b.Lines[index.AsInt64()] = line
	return Ok(b)
}

func BufferToText(b *Buffer) string { return strings.Join(b.Lines, "\n") }

// Stack is Zhex Stack<T>: a shared-mutable LIFO whose top is the end of
// the slice. There are no traversal words in v1, so there is no Active
// guard to keep.
type Stack[T any] struct {
	Items []T
}

func NewStack[T any]() *Stack[T] { return &Stack[T]{} }

func stackRange[T any](pos Pos, operation, index string, length int) Result[T] {
	return collectionFailure[T](pos, "stack-range", operation, "stack index is out of range", map[string]string{"index": index, "length": strconv.Itoa(length)})
}

// StackPush appends the new top and returns the receiver — the
// mutation-words contract, list.append's shape.
func StackPush[T any](pos Pos, s *Stack[T], item T) Result[*Stack[T]] {
	if len(s.Items) >= MaxElements {
		return collectionFailure[*Stack[T]](pos, "collection-limit", "stack.push", "stack exceeds element limit", nil)
	}
	s.Items = append(s.Items, item)
	return Ok(s)
}

// StackPop removes and returns the top — the one sanctioned exception to
// mutation words returning their collection. The empty failure keeps the
// family diagnostic: index -1 against length 0.
func StackPop[T any](pos Pos, s *Stack[T]) Result[T] {
	if len(s.Items) == 0 {
		return stackRange[T](pos, "stack.pop", "-1", 0)
	}
	last := s.Items[len(s.Items)-1]
	s.Items = s.Items[:len(s.Items)-1]
	return Ok(last)
}

// StackPeek reads the top without removing it; the empty failure mirrors
// the pop.
func StackPeek[T any](pos Pos, s *Stack[T]) Result[T] {
	if len(s.Items) == 0 {
		return stackRange[T](pos, "stack.peek", "-1", 0)
	}
	return Ok(s.Items[len(s.Items)-1])
}

func StackLength[T any](s *Stack[T]) Int { return I64(int64(len(s.Items))) }

// StackDup, StackSwap, and StackDrop are the Forth ops: each returns the
// receiver, failing stack-range when the top (or the top pair for swap)
// is missing.
func StackDup[T any](pos Pos, s *Stack[T]) Result[*Stack[T]] {
	if len(s.Items) == 0 {
		return stackRange[*Stack[T]](pos, "stack.dup", "-1", 0)
	}
	if len(s.Items) >= MaxElements {
		return collectionFailure[*Stack[T]](pos, "collection-limit", "stack.dup", "stack exceeds element limit", nil)
	}
	s.Items = append(s.Items, s.Items[len(s.Items)-1])
	return Ok(s)
}

func StackSwap[T any](pos Pos, s *Stack[T]) Result[*Stack[T]] {
	if len(s.Items) < 2 {
		return stackRange[*Stack[T]](pos, "stack.swap", "-2", len(s.Items))
	}
	n := len(s.Items)
	s.Items[n-1], s.Items[n-2] = s.Items[n-2], s.Items[n-1]
	return Ok(s)
}

func StackDrop[T any](pos Pos, s *Stack[T]) Result[*Stack[T]] {
	if len(s.Items) == 0 {
		return stackRange[*Stack[T]](pos, "stack.drop", "-1", 0)
	}
	s.Items = s.Items[:len(s.Items)-1]
	return Ok(s)
}

// StackStringInt, StackStringString, and StackStringBool render the print
// form: elements bottom-to-top in square brackets — the list literal's
// shape, the top last — with String members quoted, the set convention.
func StackStringInt(s *Stack[Int]) string {
	return stackBrackets(s.Items, func(v Int) string { return Str(v) })
}

func StackStringString(s *Stack[string]) string {
	return stackBrackets(s.Items, func(v string) string { return strconv.Quote(v) })
}

func StackStringBool(s *Stack[bool]) string {
	return stackBrackets(s.Items, func(v bool) string { return strconv.FormatBool(v) })
}

func stackBrackets[T any](items []T, show func(T) string) string {
	parts := make([]string, len(items))
	for i, v := range items {
		parts[i] = show(v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
