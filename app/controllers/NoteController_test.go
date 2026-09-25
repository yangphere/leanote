package controllers

import "testing"

func TestAllBlogResultsSucceededRequiresNonEmptyAllTrue(t *testing.T) {
	tests := []struct {
		name    string
		results []bool
		want    bool
	}{
		{name: "empty", results: nil, want: false},
		{name: "all succeeded", results: []bool{true, true}, want: true},
		{name: "one failed", results: []bool{true, false}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := allBlogResultsSucceeded(test.results); got != test.want {
				t.Fatalf("allBlogResultsSucceeded(%v) = %v, want %v", test.results, got, test.want)
			}
		})
	}
}
