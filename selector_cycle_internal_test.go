package ognl

import (
	"reflect"
	"testing"
)

type selectorInternalPointerCycle *selectorInternalPointerCycle

func TestSelectorContainerKindsStopsAtNilNamedPointer(t *testing.T) {
	var value selectorInternalPointerCycle
	kind, keyKind := selectorContainerKinds(reflect.TypeOf(value), reflect.ValueOf(value))
	if kind != reflect.Ptr || keyKind != reflect.Invalid {
		t.Fatalf("nil named pointer classification = (%s, %s), want (%s, %s)", kind, keyKind, reflect.Ptr, reflect.Invalid)
	}
}
