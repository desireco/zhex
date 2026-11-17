//go:build js

package main

import (
	"bytes"
	"errors"
	"strings"
	"syscall/js"

	"zhex/internal/lang"
)

// resolver is the host's `use` supplier, installed with setResolve.
var resolver js.Value

func main() {
	api := js.Global().Get("Object").New()
	api.Set("runSource", js.FuncOf(runSource))
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
