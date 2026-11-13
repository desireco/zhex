package lang

import "strconv"

func failedCollection(pos position, code Code, operation, message string, details map[string]string) value {
	d := errorAt(pos, code, "%s", message).(*diagnostic)
	d.operation, d.details = operation, details
	return value{result: &inputResult{err: d}}
}

func listRead(pos position, list *listValue, index int64) value {
	if index < 0 || index >= int64(len(list.items)) {
		return failedCollection(pos, CodeListRange, "list.index", "list index is out of range", map[string]string{"index": strconv.FormatInt(index, 10), "length": strconv.Itoa(len(list.items))})
	}
	return value{result: &inputResult{value: list.items[index]}}
}

func listWrite(pos position, list *listValue, index int64, item value) value {
	if list.active > 0 {
		return failedCollection(pos, CodeCollectionIterating, "list.set", "collection is being iterated", nil)
	}
	if index < 0 || index >= int64(len(list.items)) {
		return failedCollection(pos, CodeListRange, "list.set", "list index is out of range", map[string]string{"index": strconv.FormatInt(index, 10), "length": strconv.Itoa(len(list.items))})
	}
	list.items[index] = item
	return value{result: &inputResult{}}
}

func arrayShape(pos position, dims *listValue, operation string) ([]int64, int, value) {
	if len(dims.items) < 2 || len(dims.items) > 32 {
		return nil, 0, failedCollection(pos, CodeArrayRank, operation, "array rank must be between 2 and 32", nil)
	}
	shape := make([]int64, len(dims.items))
	zero := false
	for i, v := range dims.items {
		shape[i] = v.number
		if v.number < 0 {
			return nil, 0, failedCollection(pos, CodeArrayShape, operation, "array extents cannot be negative", nil)
		}
		zero = zero || v.number == 0
	}
	if zero {
		return shape, 0, value{}
	}
	count := int64(1)
	for _, n := range shape {
		if n > maxCollectionElements/count {
			return nil, 0, failedCollection(pos, CodeCollectionLimit, operation, "array exceeds element limit", nil)
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
func arrayIndex(pos position, a *arrayValue, indices []int64, item value, write bool) value {
	operation := "array.index"
	if write {
		operation = "array.set"
		if a.active > 0 {
			return failedCollection(pos, CodeCollectionIterating, operation, "collection is being iterated", nil)
		}
	}
	if len(indices) != len(a.shape) {
		return failedCollection(pos, CodeArrayRank, operation, "array index has the wrong rank", nil)
	}
	offset := int64(0)
	for i, n := range indices {
		if n < 0 || n >= a.shape[i] {
			return failedCollection(pos, CodeArrayRange, operation, "array coordinate is out of range", nil)
		}
		offset = offset*a.shape[i] + n
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
		return failedCollection(pos, CodeArrayRank, "array.resize", "array resize cannot change rank", nil)
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
	return value{result: &inputResult{}}
}
