"use strict";
// Differential driver for the browser engine build (#91). One JSON answer on
// stdout per invocation:
//   node engine.cjs <wasm_exec.js> <module.wasm> run <source.zhex> <stdin.txt>
//   node engine.cjs <wasm_exec.js> <module.wasm> probes <sources.json>
//   node engine.cjs <wasm_exec.js> <module.wasm> session <job.json>
//   node engine.cjs <wasm_exec.js> <module.wasm> interactive <job.json>
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
  } else if (mode === "interactive") {
    // The async stdin seam (#122): runInteractive starts the program on its own
    // goroutine; lines are delivered (after a settle window by default, or
    // eagerly before the program parks) and close ends the stream. The answer
    // reports every output chunk, the chunks visible while the program was
    // parked waiting for input, and the JS event-loop lag across the settle
    // window — a parked program leaves the loop free.
    const job = JSON.parse(fs.readFileSync(path.resolve(args[0]), "utf8"));
    const chunks = [];
    let chunksAtSettle = null;
    let result = null;
    const handle = zhex.runInteractive(job.source, (text) => chunks.push(text), (r) => { result = r; });
    const deliverAll = () => {
      for (const line of job.lines || []) handle.deliver(line);
      if (job.close !== false) handle.close();
    };
    if (job.eager) {
      deliverAll();
      chunksAtSettle = chunks.slice();
    }
    const t0 = Date.now();
    await new Promise((resolve) => setTimeout(resolve, job.settleMs ?? 50));
    const loopLagMs = Date.now() - t0;
    if (!job.eager) {
      chunksAtSettle = chunks.slice();
      deliverAll();
    }
    const deadline = Date.now() + 10000;
    while (!result && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 5));
    if (!result) throw new Error("interactive run never finished");
    let lateDeliverMs = null;
    let lateDeliverError = null;
    if (job.lateDeliver) {
      const late = Date.now();
      try {
        for (let i = 0; i < job.lateDeliver; i++) handle.deliver("late");
      } catch (e) {
        lateDeliverError = String(e);
      }
      lateDeliverMs = Date.now() - late;
    }
    console.log(JSON.stringify({ chunks, chunksAtSettle, loopLagMs, lateDeliverMs, lateDeliverError, result }));
  } else {
    throw new Error("unknown mode " + mode);
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
