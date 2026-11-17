// Zhex browser engine loader (#91) — the one entry every consumer shares
// (the #88 playground, the read-line spike harness). Ships beside the
// artifact trio it loads, all produced by the justfile's engine recipe:
//   zhex-engine.wasm      the interpreter compiled to WebAssembly
//   zhex-engine-exec.js   Go's wasm_exec runtime
//   zhex-engine.js        this loader
//
//   import { loadEngine } from "/zhex-engine.js";
//   const zhex = await loadEngine();
//   const run = zhex.runSource('print("hello")', "");
//   run.output; // "hello\n"
//
// The full API is documented in cmd/zhex/engine/doc.go: runSource (one-shot,
// optional third argument turns on the JSONL trace), newSession (the REPL
// seam), the continuation probes, and setResolve for `use` module sources.
// Load the engine lazily behind a click — the module is megabytes.
export async function loadEngine(base = "/") {
  if (!globalThis.Go) {
    await new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = base + "zhex-engine-exec.js";
      script.onload = resolve;
      script.onerror = () => reject(new Error("cannot load " + script.src));
      document.head.appendChild(script);
    });
  }
  const go = new globalThis.Go();
  const bytes = await (await fetch(base + "zhex-engine.wasm")).arrayBuffer();
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  // Not awaited: the engine's main blocks forever serving the API, so go.run
  // never resolves. Readiness is globalThis.zhex appearing.
  go.run(instance);
  while (!globalThis.zhex) await new Promise((resolve) => setTimeout(resolve, 0));
  return globalThis.zhex;
}
