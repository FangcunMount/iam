package options

import (
	"reflect"
	"testing"
)

func TestObserveCompatibilityConfigKeysReturnsOnlyPresentFixedKeys(t *testing.T) {
	present := ObserveCompatibilityConfigKeys(func(key string) bool {
		return key == compatibilityKeyLoaderPlaceholderTenantID
	})

	want := []string{compatibilityKeyLoaderPlaceholderTenantID}
	if !reflect.DeepEqual(present, want) {
		t.Fatalf("present keys = %#v, want %#v", present, want)
	}
}
