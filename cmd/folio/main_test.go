package main

import (
	"reflect"
	"testing"
)

func TestParsePages(t *testing.T) {
	cases := []struct {
		spec  string
		total int
		want  []int
		err   bool
	}{
		{"", 3, []int{1, 2, 3}, false},
		{"1", 3, []int{1}, false},
		{"2-3", 3, []int{2, 3}, false},
		{"1-2,3", 3, []int{1, 2, 3}, false},
		{"3,1,2", 3, []int{1, 2, 3}, false}, // de-duped + sorted
		{"1-1", 3, []int{1}, false},
		{"5-2", 5, []int{2, 3, 4, 5}, false}, // reversed range normalized
		{"0", 3, nil, true},                  // out of range (low)
		{"4", 3, nil, true},                  // out of range (high)
		{"abc", 3, nil, true},                // not a number
	}
	for _, c := range cases {
		got, err := parsePages(c.spec, c.total)
		if c.err {
			if err == nil {
				t.Errorf("parsePages(%q): expected error, got %v", c.spec, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePages(%q): unexpected error: %v", c.spec, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parsePages(%q) = %v, want %v", c.spec, got, c.want)
		}
	}
}
