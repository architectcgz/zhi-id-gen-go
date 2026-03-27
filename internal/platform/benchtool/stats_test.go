package benchtool

import "testing"

func TestCountDuplicates(t *testing.T) {
	testCases := []struct {
		name string
		ids  []int64
		want int
	}{
		{name: "empty", ids: nil, want: 0},
		{name: "unique", ids: []int64{3, 1, 2}, want: 0},
		{name: "one duplicate", ids: []int64{5, 1, 5}, want: 1},
		{name: "multiple duplicates", ids: []int64{2, 2, 2, 3, 3}, want: 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := CountDuplicates(tc.ids)
			if got != tc.want {
				t.Fatalf("CountDuplicates(%v) = %d, want %d", tc.ids, got, tc.want)
			}
		})
	}
}
