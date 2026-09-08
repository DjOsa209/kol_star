package main

import "testing"

func TestAllocateCooperationCosts(t *testing.T) {
	tests := []struct {
		name  string
		total float64
		count int
		want  []float64
	}{
		{name: "single", total: 1200, count: 1, want: []float64{1200}},
		{name: "package exact", total: 900, count: 3, want: []float64{300, 300, 300}},
		{name: "package keeps cents", total: 1000, count: 3, want: []float64{333.34, 333.33, 333.33}},
		{name: "empty", total: 100, count: 0, want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := allocateCooperationCosts(test.total, test.count)
			if len(got) != len(test.want) {
				t.Fatalf("len = %d, want %d", len(got), len(test.want))
			}
			for index := range test.want {
				if got[index] != test.want[index] {
					t.Fatalf("cost[%d] = %.2f, want %.2f", index, got[index], test.want[index])
				}
			}
		})
	}
}
