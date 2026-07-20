package codegen_test

import (
	"strings"
	"testing"
)

func TestPrintBuiltinEmitsNamespacedCall(t *testing.T) {
	src := `test ("prints") {
  print("hi")
}`
	script := compileTest(t, "print_test.wisp", src)
	if !strings.Contains(string(script), "__wisp_print ") {
		t.Errorf("generated script does not call __wisp_print:\n%s", script)
	}
	if strings.Contains(string(script), "\nprint ") {
		t.Errorf("generated script still calls bare print:\n%s", script)
	}
}
