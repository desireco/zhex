// The playground's engine worker (#224): the whole #91 engine lives on this
// worker instead of the page's thread, so a program that never ends cannot
// freeze the page. Stop ends the worker; the page starts a fresh one for the
// next run. Nothing here touches the DOM — the page talks in messages, this
// script answers them — and the engine loads through the shipped loader
// (zhex-engine.js, cmd/zhex/engine/loader.js), never a copy of its steps.
//
// Page → worker (every answer carries the request's id):
//   {type: "init", base}              load the engine from base → {type: "ready"}
//                                     or {type: "load-failed"}
//   {type: "resolve", modules}        the `use` sources behind setResolve
//   {type: "runSource", source, stdin}  the one-shot run → {id, type: "result", result}
//   {type: "runInteractive", source}    the live run → {id, type: "started"}, then
//                                     {id, type: "output", text} per write and
//                                     {id, type: "result", result} when it ends
//   {type: "deliver", line}           one typed line for the live run
//   {type: "closeInput"}              end the live run's input
//   {type: "newSession", stdin}       the REPL session → {id, type: "session"}
//   {type: "enter", source}           one REPL entry → {id, type: "entry", result,
//                                     output, diagnostics}
//   {type: "words"}                   session.words() → {id, type: "words", list}
//   {type: "probe", source}           the continuation probes → {id, type: "probe",
//                                     probe: {incomplete, delimiter, lastStatement}}
// Messages that arrive while the engine still loads wait in a queue and drain
// in order once it answers.

let engine = null;
let session = null;
let liveHandle = null;
let liveId = null;
let outputBatch = "";
let outputTimer = null;
let modules = {};
let loaded = false;
const queue = [];

const post = message => self.postMessage(message);

async function init(base) {
  try {
    // The shipped loader, by the absolute base the page sends; inside a
    // classic worker it runs the runtime with importScripts.
    const { loadEngine } = await import(base + "zhex-engine.js");
    engine = await loadEngine(base);
    engine.setResolve(name => modules[name] ? { path: `${name}.zhex`, source: modules[name] } : null);
    post({ type: "ready" });
  } catch {
    post({ type: "load-failed" });
    return;
  }
  loaded = true;
  for (const message of queue) handle(message);
  queue.length = 0;
}

function handle(message) {
  try {
    switch (message.type) {
      case "resolve":
        modules = message.modules || {};
        return;
      case "runSource":
        post({ id: message.id, type: "result", result: engine.runSource(message.source, message.stdin) });
        return;
      case "runInteractive": {
        const id = message.id;
        // Started goes out first: the program's own prompt can be written
        // while runInteractive is still on the stack, and the page waits
        // for started before it treats later messages as stream events.
        post({ id, type: "started" });
        liveId = id;
        outputBatch = "";
        // Output batches on a short timer: a program that prints in a loop
        // would otherwise post a message per line and drown the page's
        // event queue, and the whole point of the worker is that the page
        // stays usable (#224). The final flush keeps output ahead of result.
        liveHandle = engine.runInteractive(
          message.source,
          text => {
            outputBatch += text;
            if (outputTimer === null) {
              outputTimer = setTimeout(() => {
                outputTimer = null;
                if (outputBatch) post({ id: liveId, type: "output", text: outputBatch });
                outputBatch = "";
              }, 30);
            }
          },
          result => {
            if (outputTimer !== null) { clearTimeout(outputTimer); outputTimer = null; }
            if (outputBatch) { post({ id: liveId, type: "output", text: outputBatch }); outputBatch = ""; }
            liveHandle = null;
            post({ id, type: "result", result });
          },
        );
        return;
      }
      case "deliver":
        if (liveHandle) liveHandle.deliver(message.line);
        return;
      case "closeInput":
        if (liveHandle) liveHandle.close();
        return;
      case "newSession":
        session = engine.newSession(message.stdin);
        post({ id: message.id, type: "session" });
        return;
      case "enter": {
        const result = session.enter(message.source);
        const output = session.takeOutput();
        const diagnostics = session.takeDiagnostics();
        post({ id: message.id, type: "entry", result, output, diagnostics });
        return;
      }
      case "words":
        post({ id: message.id, type: "words", list: session.words() });
        return;
      case "probe":
        post({
          id: message.id,
          type: "probe",
          probe: {
            incomplete: engine.isIncomplete(message.source),
            delimiter: engine.delimiterIndent(message.source),
            lastStatement: engine.lastStatementIndent(message.source),
          },
        });
        return;
    }
  } catch (failure) {
    post({ id: message.id, type: "error", message: String(failure && failure.message ? failure.message : failure) });
  }
}

self.onmessage = event => {
  const message = event.data;
  if (message.type === "init") {
    init(message.base);
    return;
  }
  if (!loaded) {
    queue.push(message);
    return;
  }
  handle(message);
};
