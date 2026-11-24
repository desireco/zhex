package lang

import (
	"sort"
	"strconv"
	"zhex/internal/zhexrt"
)

func isCheckedCollectionOperation(name string) bool {
	return name == "array.new" || name == "array.resize" || name == "list.append" || name == "list.insert" || name == "list.pop" || name == "list.first" || name == "list.last" || name == "file.exists"
}

// sortListStable returns a fresh list of items in ascending key order; equal
// keys keep source order. Keys are precomputed by the caller, evaluated once
// per element in source order before any comparison.
func sortValuesStable(items []value, byText bool) []value {
	sorted := append([]value(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if byText {
			return sorted[i].text < sorted[j].text
		}
		return zhexrt.Cmp(sorted[i].number, sorted[j].number) < 0
	})
	return sorted
}

func sortListStable(items, keys []value, byText bool) []value {
	order := make([]int, len(items))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		if byText {
			return keys[order[a]].text < keys[order[b]].text
		}
		return zhexrt.Cmp(keys[order[a]].number, keys[order[b]].number) < 0
	})
	sorted := make([]value, len(items))
	for i, at := range order {
		sorted[i] = items[at]
	}
	return sorted
}

func failedCollection(pos position, code Code, operation, message string, details map[string]string) value {
	d := errorAt(pos, code, "%s", message).(*diagnostic)
	d.operation, d.details = operation, details
	return value{result: &inputResult{err: d}}
}

func listRead(pos position, list *listValue, index i128) value {
	if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() >= int64(len(list.items)) {
		return failedCollection(pos, CodeListRange, "list.index", "list index is out of range", map[string]string{"index": zhexrt.Str(index), "length": strconv.Itoa(len(list.items))})
	}
	return value{result: &inputResult{value: list.items[index.AsInt64()]}}
}

func listWrite(pos position, list *listValue, index i128, item value) value {
	if list.active > 0 {
		return failedCollection(pos, CodeCollectionIterating, "list.set", "collection is being iterated", nil)
	}
	if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() >= int64(len(list.items)) {
		return failedCollection(pos, CodeListRange, "list.set", "list index is out of range", map[string]string{"index": zhexrt.Str(index), "length": strconv.Itoa(len(list.items))})
	}
	list.items[index.AsInt64()] = item
	return value{result: &inputResult{}}
}

func arrayShape(pos position, dims *listValue, operation string) ([]int64, int, value) {
	if len(dims.items) < 1 || len(dims.items) > 32 {
		return nil, 0, failedCollection(pos, CodeArrayRank, operation, "array rank must be between 1 and 32", map[string]string{"rank": strconv.Itoa(len(dims.items)), "min_rank": "1", "max_rank": "32"})
	}
	shape := make([]int64, len(dims.items))
	zero := false
	for i, v := range dims.items {
		if v.number.Negative() {
			return nil, 0, failedCollection(pos, CodeArrayShape, operation, "array extents cannot be negative", map[string]string{"axis": strconv.Itoa(i), "extent": zhexrt.Str(v.number)})
		}
		if !v.number.Fits64() {
			return nil, 0, failedCollection(pos, CodeCollectionLimit, operation, "array exceeds element limit", map[string]string{"limit": strconv.Itoa(maxCollectionElements)})
		}
		shape[i] = v.number.AsInt64()
		zero = zero || v.number.IsZero()
	}
	if zero {
		return shape, 0, value{}
	}
	count := int64(1)
	for _, n := range shape {
		if n > maxCollectionElements/count {
			return nil, 0, failedCollection(pos, CodeCollectionLimit, operation, "array exceeds element limit", map[string]string{"limit": strconv.Itoa(maxCollectionElements)})
		}
		count *= n
	}
	return shape, int(count), value{}
}
func arrayNew(pos position, dims *listValue, fill value) value {
	shape, count, failure := arrayShape(pos, dims, "array.new")
	if failure.result != nil {
		return failure
	}
	items := make([]value, count)
	for i := range items {
		items[i] = fill
	}
	return value{result: &inputResult{value: value{array: &arrayValue{shape: shape, items: items}}}}
}
func arrayIndex(pos position, a *arrayValue, indices []i128, item value, write bool) value {
	operation := "array.index"
	if write {
		operation = "array.set"
		if a.active > 0 {
			return failedCollection(pos, CodeCollectionIterating, operation, "collection is being iterated", nil)
		}
	}
	if len(indices) != len(a.shape) {
		return failedCollection(pos, CodeArrayRank, operation, "array index has the wrong rank", map[string]string{"rank": strconv.Itoa(len(indices)), "expected_rank": strconv.Itoa(len(a.shape))})
	}
	offset := int64(0)
	for i, index := range indices {
		if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() >= a.shape[i] {
			return failedCollection(pos, CodeArrayRange, operation, "array coordinate is out of range", map[string]string{"axis": strconv.Itoa(i), "index": zhexrt.Str(index), "extent": strconv.FormatInt(a.shape[i], 10)})
		}
		offset = offset*a.shape[i] + index.AsInt64()
	}
	if write {
		a.items[offset] = item
		return value{result: &inputResult{}}
	}
	return value{result: &inputResult{value: a.items[offset]}}
}
func arrayResize(pos position, a *arrayValue, dims *listValue, fill value) value {
	if a.active > 0 {
		return failedCollection(pos, CodeCollectionIterating, "array.resize", "collection is being iterated", nil)
	}
	shape, count, failure := arrayShape(pos, dims, "array.resize")
	if failure.result != nil {
		return failure
	}
	if len(shape) != len(a.shape) {
		return failedCollection(pos, CodeArrayRank, "array.resize", "array resize cannot change rank", map[string]string{"rank": strconv.Itoa(len(shape)), "expected_rank": strconv.Itoa(len(a.shape))})
	}
	items := make([]value, count)
	for i := range items {
		items[i] = fill
	}
	for old, v := range a.items {
		index := int64(old)
		offset := int64(0)
		stride := int64(1)
		fits := true
		for axis := len(shape) - 1; axis >= 0; axis-- {
			coord := index % a.shape[axis]
			index /= a.shape[axis]
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
	a.shape, a.items = shape, items
	return value{result: &inputResult{value: value{array: a}}}
}

// plainWriteResult promotes checked collection failures to runtime diagnostics.
func plainWriteResult(result value) (value, error) {
	if result.result.err != nil {
		return value{}, result.result.err
	}
	return value{}, nil
}

func plainReadResult(result value) (value, error) {
	if result.result.err != nil {
		return value{}, result.result.err
	}
	return result.result.value, nil
}
