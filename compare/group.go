package compare

type duplicateGroup[T any] struct {
	ListDuplicated []T
}

func addToGroup[K comparable, T any](groups map[K]duplicateGroup[T], key K, item T) {
	group := groups[key]
	group.ListDuplicated = append(group.ListDuplicated, item)
	groups[key] = group
}

func removeSingletonGroups[K comparable, T any](groups map[K]duplicateGroup[T]) {
	for key, group := range groups {
		if len(group.ListDuplicated) < 2 {
			delete(groups, key)
		}
	}
}

func countItemsInGroups[K comparable, T any](groups map[K]duplicateGroup[T]) int {
	count := 0
	for _, group := range groups {
		count += len(group.ListDuplicated)
	}
	return count
}

func collectItemsInGroups[K comparable, T any](groups map[K]duplicateGroup[T]) []T {
	var items []T
	for _, group := range groups {
		items = append(items, group.ListDuplicated...)
	}
	return items
}
