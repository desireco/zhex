package zhexrt

import (
	"os"
	"testing"
)

// TestRunRendersStrayPanics: no Go panic ever leaks raw — an unexpected
// runtime panic renders as a Zhex diagnostic naming a native-internal
// code and exits 1 (#185 M4).
func TestRunRendersStrayPanics(t *testing.T) {
	if os.Getenv("ZHEXRT_PANIC_CHILD") == "1" {
		Sources["boom.zhex"] = "print(1 / 0)\n"
		Run(func() *Error { panic("surprise") })
	}
	cmd := os.Args[0]
	env := append(os.Environ(), "ZHEXRT_PANIC_CHILD=1")
	proc, err := os.StartProcess(cmd, []string{cmd}, &os.ProcAttr{Env: env, Files: []*os.File{nil, nil, os.Stderr}})
	if err != nil {
		t.Skip("cannot re-exec:", err)
	}
	state, err := proc.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if code := state.ExitCode(); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
}
