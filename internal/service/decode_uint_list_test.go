package service

import (
	"reflect"
	"testing"
)

func TestDecodeUintListCanonical(t *testing.T) {
	values, err := decodeUintList(`[3,1,3,2]`)
	if err != nil {
		t.Fatal(err)
	}
	expected := []uint{1, 2, 3}
	if !reflect.DeepEqual(values, expected) {
		t.Fatalf("decodeUintList must dedup and sort: got %v want %v", values, expected)
	}
}
