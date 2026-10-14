package main

import (
	"reflect"
	"testing"
)

func TestCompare(t *testing.T) {
	shared, same, oldOnly, newOnly := compare([]string{"a", "b", "c", "d"}, []string{"a", "c", "b", "e"})
	if shared != 3 || same != 1 || !reflect.DeepEqual(oldOnly, []string{"4:d"}) || !reflect.DeepEqual(newOnly, []string{"4:e"}) {
		t.Fatalf("compare = %d %d %v %v", shared, same, oldOnly, newOnly)
	}
}
