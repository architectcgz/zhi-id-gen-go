package benchtool

import "sort"

func CountDuplicates(ids []int64) int {
	if len(ids) < 2 {
		return 0
	}

	sorted := append([]int64(nil), ids...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	duplicates := 0
	for i := 1; i < len(sorted); i++ {
		if sorted[i] == sorted[i-1] {
			duplicates++
		}
	}
	return duplicates
}
