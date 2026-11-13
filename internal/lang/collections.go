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
