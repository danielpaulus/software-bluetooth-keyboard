package hid

import (
	"fmt"
	"strings"
	"testing"
)

func TestKeymapConvert(t *testing.T) {
	test := "test"
	for _, char := range test {
		key := strings.ToUpper(fmt.Sprintf("KEY_%c", char))
		if k, mk := Convert(key); mk != FUNC {
			t.Error("KEY_A is not a modifier key: got ", mk, k)
		}
	}
	if k, mk := Convert("KEY_A"); mk == MOD {
		t.Error("KEY_A is not a modifier key: got ", mk, k)
	}

	if k, mk := Convert("KEY_RIGHTMETA"); mk == FUNC {
		t.Error("KEY_RIGHTMETA is not a function key: got ", mk, k)
	}
}
