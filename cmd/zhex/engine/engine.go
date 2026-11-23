//go:build js

package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"syscall/js"

	"zhex/internal/lang"
)

// resolver is the host's `use` supplier, installed with setResolve.
var resolver js.Value

func main() {
	api := js.Global().Get("Object").New()
	api.Set("runSource", js.FuncOf(runSource))
	api.Set("runInteractive", js.FuncOf(runInteractive))
	api.Set("newSession", js.FuncOf(newSession))
	api.Set("isIncomplete", js.FuncOf(func(_ js.Value, args []js.Value) any {
		_, err := lang.Compile("repl", text(args, 0))
		return lang.IsIncomplete(err)
	}))
	api.Set("delimiterIndent", js.FuncOf(func(_ js.Value, args []js.Value) any {
		source := text(args, 0)
		_, err := lang.Compile("repl", source)
		indent, open := lang.DelimiterIndent(source, err)
		return optionalInt(indent, open)
	}))
	api.Set("lastStatementIndent", js.FuncOf(func(_ js.Value, args []js.Value) any {
		indent, ok := lang.LastStatementIndent(text(args, 0))
		return optionalInt(indent, ok)
	}))
	api.Set("setResolve", js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolver = args[0]
		return nil
	}))
	js.Global().Set("zhex", api)
	// Keep the Go runtime alive so the registered functions keep answering;
	// a returned main exits the module and every later call would fail.
	select {}
}

// resolveModule adapts the host resolver to the engine seam's signature; a
// missing resolver or a falsy answer rejects the use with use-missing.
func resolveModule(name string) (lang.ModuleSource, bool) {
	if !resolver.Truthy() {
		return lang.ModuleSource{}, false
	}
	source := resolver.Invoke(name)
	if !source.Truthy() {
		return lang.ModuleSource{}, false
	}
	return lang.ModuleSource{Path: source.Get("path").String(), Source: source.Get("source").String()}, true
}

// runSource is the one-shot seam: one program, one supplied stdin string,
// every channel captured for the host. The optional third argument turns on
// the trace, mirroring the CLI's opt-in --trace — a traced Life demo emits
// gigabytes of JSONL, so it never runs by default.
func runSource(_ js.Value, args []js.Value) any {
	var out, diagnostics, trace bytes.Buffer
	options := lang.RunOptions{Diagnostics: &diagnostics}
	if len(args) > 2 && args[2].Bool() {
		options.Trace = &trace
	}
	err := lang.RunWithModulesOptions("playground", text(args, 0), strings.NewReader(text(args, 1)), &out, options, resolveModule)
	return finishResult(err, &out, &diagnostics, &trace)
}

// finishResult shapes the one-shot answer: program output, the console.fail
// channel, the --report=json envelope of a failed run (nil otherwise), the
// trace text, and the status zhex run would exit with.
func finishResult(err error, out, diagnostics, trace *bytes.Buffer) js.Value {
	result := js.Global().Get("Object").New()
	result.Set("output", out.String())
	result.Set("diagnostics", diagnostics.String())
	result.Set("trace", trace.String())
	exit, report := 0, any(nil)
	if err != nil {
		var requested *lang.ExitError
		if errors.As(err, &requested) {
			exit = requested.Status
		} else {
			exit = 1
			if encoded, ok := lang.Report(err); ok {
				report = encoded
			}
		}
	}
	result.Set("exit", exit)
	result.Set("report", report)
	return result
}

// interactiveInput is the async stdin (#122): Read parks on host-delivered
// lines until the host closes the stream, the same shape of wait a blocking
// console read takes. Read runs on the program goroutine only; deliver and
// close run on JS callback goroutines. shortcut: the line channel holds 16
// undelivered lines, far past user typing pace — a host streaming bulk input
// through it parks its deliver callback until the program drains, but never
// past close or the run's end (select on done below), so no callback parks
// forever.
type interactiveInput struct {
	pending []byte
	lines   chan []byte
	done    chan struct{}
	once    sync.Once
}

func newInteractiveInput() *interactiveInput {
	return &interactiveInput{lines: make(chan []byte, 16), done: make(chan struct{})}
}

// deliver queues one line; a missing trailing newline is added, so a typed
// line reads back exactly as a terminated line of piped stdin. Once the
// stream has ended the line is dropped — a late deliver returns instead of
// parking its goroutine on a full channel nobody will drain.
func (in *interactiveInput) deliver(line string) {
	if line == "" || line[len(line)-1] != '\n' {
		line += "\n"
	}
	select {
	case in.lines <- []byte(line):
	case <-in.done:
	}
}

// close ends the stream: queued lines still deliver first, then reads see
// end-of-file.
func (in *interactiveInput) close() {
	in.once.Do(func() { close(in.done) })
}

func (in *interactiveInput) Read(p []byte) (int, error) {
	for len(in.pending) == 0 {
		select {
		case in.pending = <-in.lines:
		case <-in.done:
			select { // close raced queued lines: deliver them before EOF
			case in.pending = <-in.lines:
			default:
				return 0, io.EOF
			}
		}
	}
	n := copy(p, in.pending)
	in.pending = in.pending[n:]
	return n, nil
}

// forwardedOutput streams every write to the host as it happens, so a prompt
// lands before the read that follows it parks, while keeping the accumulated
// text the result reports.
type forwardedOutput struct {
	onOutput js.Value
	buf      bytes.Buffer
}

func (w *forwardedOutput) Write(p []byte) (int, error) {
	w.onOutput.Invoke(string(p))
	return w.buf.Write(p)
}

// runInteractive is the async seam (#122): like runSource, but the program
// runs on its own goroutine with read-line parking on host-delivered input
// instead of draining a supplied string. onOutput(text) receives each write
// as it happens; onDone(result) receives runSource's result shape once the
// program finishes. The returned handle's deliver(line) queues one line and
// close() ends the stream — a read past close fails with the coded
// end-of-file failure, matching runSource's supplied-stdin exhaustion. A
// caller abandoning a run must call close(): it unparks a blocked read as
// end-of-file, lets the run goroutine finish, and Releases the handle's
// two js.Func wrappers (the run's exit Releases them too, whichever comes
// first) — otherwise the goroutine and the wrappers stay for the module's
// lifetime.
func runInteractive(_ js.Value, args []js.Value) any {
	source, onOutput, onDone := text(args, 0), args[1], args[2]
	in := newInteractiveInput()
	deliverFn := js.FuncOf(func(_ js.Value, callArgs []js.Value) any {
		in.deliver(text(callArgs, 0))
		return nil
	})
	var closeFn js.Func
	var released sync.Once
	release := func() {
		released.Do(func() {
			deliverFn.Release()
			closeFn.Release()
		})
	}
	closeFn = js.FuncOf(func(js.Value, []js.Value) any {
		in.close()
		release()
		return nil
	})
	handle := js.Global().Get("Object").New()
	handle.Set("deliver", deliverFn)
	handle.Set("close", closeFn)
	go func() {
		// Registered first, so it runs last: after onDone and the input
		// close, the wrappers are freed even when the host never calls
		// close().
		defer release()
		defer in.close()
		out := &forwardedOutput{onOutput: onOutput}
		var diagnostics bytes.Buffer
		err := lang.RunWithModulesOptions("playground", source, in, out, lang.RunOptions{Diagnostics: &diagnostics}, resolveModule)
		onDone.Invoke(finishResult(err, &out.buf, &diagnostics, &bytes.Buffer{}))
	}()
	return handle
}

// newSession is the REPL seam: one session with its own stdin string and
// buffered output and diagnostics channels the host drains.
func newSession(_ js.Value, args []js.Value) any {
	var out, diagnostics bytes.Buffer
	session := lang.NewSession(strings.NewReader(text(args, 0)), &out, nil, resolveModule)
	session.SetDiagnostics(&diagnostics)
	obj := js.Global().Get("Object").New()
	obj.Set("enter", js.FuncOf(func(_ js.Value, callArgs []js.Value) any {
		echo, err := session.Enter(text(callArgs, 0))
		result := js.Global().Get("Object").New()
		result.Set("echo", echo)
		result.Set("error", "")
		result.Set("report", nil)
		result.Set("exit", nil)
		if err == nil {
			return result
		}
		var requested *lang.ExitError
		if errors.As(err, &requested) {
			result.Set("exit", requested.Status)
			return result
		}
		result.Set("error", err.Error())
		if encoded, ok := lang.Report(err); ok {
			result.Set("report", encoded)
		}
		return result
	}))
	obj.Set("words", js.FuncOf(func(js.Value, []js.Value) any {
		list := js.Global().Get("Array").New()
		for _, signature := range session.Words() {
			list.Call("push", signature)
		}
		return list
	}))
	obj.Set("takeOutput", js.FuncOf(func(js.Value, []js.Value) any {
		drained := out.String()
		out.Reset()
		return drained
	}))
	obj.Set("takeDiagnostics", js.FuncOf(func(js.Value, []js.Value) any {
		drained := diagnostics.String()
		diagnostics.Reset()
		return drained
	}))
	return obj
}

// text reads args[i] as a string; a missing argument reads as "".
func text(args []js.Value, i int) string {
	if i >= len(args) {
		return ""
	}
	return args[i].String()
}

// optionalInt maps an absent probe answer to JS null.
func optionalInt(value int, ok bool) any {
	if !ok {
		return nil
	}
	return value
}
