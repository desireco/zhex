package zhexrt

import (
	"os"
	"strings"
	"testing"
)

// TestRenderDiagAppendsLearnLine pins the additive learn line in the shared
// renderer (interpreter and native runtime render byte-identically through
// it): the URL follows the hint, an empty URL adds nothing, and the native
// runtime suppresses it for uncataloged codes — application error.new
// codes and native-internal own no catalog section.
func TestRenderDiagAppendsLearnLine(t *testing.T) {
	got := RenderDiag("f.zhex", 1, 1, "boom", "do this", "print(1)", 0, 0, LearnURL("division-zero"))
	want := "f.zhex:1:1: boom\n    print(1)\n    ^\nhint: do this\nlearn: " + LearnURL("division-zero")
	if got != want {
		t.Fatalf("render:\n%s\nwant:\n%s", got, want)
	}
	if got := RenderDiag("f.zhex", 1, 1, "boom", "", "", 0, 0, ""); strings.Contains(got, "learn") {
		t.Fatalf("empty learn URL must add nothing: %q", got)
	}
	builtin := (&Error{Pos: At("f.zhex", 1, 1), Code: "division-zero", Message: "boom"}).render()
	if !strings.HasSuffix(builtin, "\nlearn: "+LearnURL("division-zero")) {
		t.Fatalf("builtin runtime code must learn: %q", builtin)
	}
	for _, uncataloged := range []*Error{
		ErrorNew(At("f.zhex", 1, 1), "app-code", "boom"),
		{Code: "native-internal", Message: "internal error: surprise", uncataloged: true},
	} {
		if rendered := uncataloged.render(); strings.Contains(rendered, "learn:") {
			t.Fatalf("uncataloged code %q must render no learn line: %q", uncataloged.Code, rendered)
		}
	}
}

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
