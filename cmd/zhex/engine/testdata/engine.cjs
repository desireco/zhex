"use strict";
// Differential driver for the browser engine build (#91). One JSON answer on
// stdout per invocation:
//   node engine.cjs <wasm_exec.js> <module.wasm> run <source.zhex> <stdin.txt>
//   node engine.cjs <wasm_exec.js> <module.wasm> probes <sources.json>
//   node engine.cjs <wasm_exec.js> <module.wasm> session <job.json>
// `use` modules resolve through the engine's host resolve hook: run mode
// reads siblings of the entry file (like zhex run), session jobs name module
// files explicitly — the host supplies module sources exactly as the
// playground will (#88 fetches the shipped siblings).
const fs = require("fs");
const path = require("path");
const [wasmExec, modulePath, mode, ...args] = process.argv.slice(2);
require(path.resolve(wasmExec));

async function main() {
  const go = new globalThis.Go();
  const { instance } = await WebAssembly.instantiate(fs.readFileSync(modulePath), go.importObject);
  // Not awaited: the engine's main blocks forever serving the API, so go.run
  // never resolves. Readiness is globalThis.zhex appearing.
  go.run(instance);
  while (!globalThis.zhex) await new Promise((resolve) => setTimeout(resolve, 0));
  const zhex = globalThis.zhex;
  if (mode === "run") {
    const trace = args.includes("--trace");
    const [sourcePath, stdinPath] = args.filter((a) => a !== "--trace").map((p) => path.resolve(p));
    const dir = path.dirname(sourcePath);
    zhex.setResolve((name) => {
      try {
        const p = path.join(dir, name + ".zhex");
        return { path: p, source: fs.readFileSync(p, "utf8") };
      } catch {
        return null;
      }
    });
    console.log(JSON.stringify(zhex.runSource(fs.readFileSync(sourcePath, "utf8"), fs.readFileSync(stdinPath, "utf8"), trace)));
  } else if (mode === "probes") {
    const results = JSON.parse(fs.readFileSync(path.resolve(args[0]), "utf8")).map((source) => ({
      incomplete: zhex.isIncomplete(source),
      delimiterIndent: zhex.delimiterIndent(source),
      lastStatementIndent: zhex.lastStatementIndent(source),
    }));
    console.log(JSON.stringify(results));
  } else if (mode === "session") {
    const job = JSON.parse(fs.readFileSync(path.resolve(args[0]), "utf8"));
    const sources = new Map(Object.entries(job.modules || {}).map(([name, p]) => [name, fs.readFileSync(path.resolve(p), "utf8")]));
    zhex.setResolve((name) => (sources.has(name) ? { path: name + ".zhex", source: sources.get(name) } : null));
    const session = zhex.newSession(job.stdin || "");
    const results = [];
    for (const entry of job.entries) {
      const r = session.enter(entry.source);
      if (entry.words) r.words = session.words();
      r.output = session.takeOutput();
      r.diagnostics = session.takeDiagnostics();
      results.push(r);
    }
    console.log(JSON.stringify(results));
  } else {
    throw new Error("unknown mode " + mode);
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
