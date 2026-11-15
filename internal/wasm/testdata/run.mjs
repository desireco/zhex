// Differential harness runner: node run.mjs <module.wasm>
// Normative for the import and trap contract in internal/wasm/DESIGN.md
// (§4 imports, §6 runtime diagnostics, §9 test strategy) and the milestone-2
// fail-code extension (DESIGN-M2 §2c): instantiate with env.print, call the
// exported zhex_main (no start section, so traps land here), print collected
// stdout, map traps to the pinned stderr strings.
import { readFile } from "node:fs/promises";

const failMessages = {
  1: "integer overflow",
  2: "word call depth limit of 256 exceeded",
  3: "list index is out of range",
  4: "array coordinate is out of range",
  5: "array rank must be between 2 and 32",
  6: "array extents cannot be negative",
  7: "array index has the wrong rank",
  8: "list exceeds element limit",
  9: "array exceeds element limit",
  10: "collection is being iterated",
  11: "unhandled failure",
  13: "text.replace old text is empty",
  14: "text.split exceeds element limit",
};

// Codes 12 and 15 stage their dynamic message as UTF-8 at memory offset 0;
// $zhex_fail_msg records the byte length for the exported zhex_msg_len()
// accessor (DESIGN-M2 §2c). Codes 13 and 14 use the fixed table above.
const stagedFailCodes = new Set([12, 15]);

const bytes = await readFile(process.argv[2]);
const decoder = new TextDecoder();
const lines = [];

const { instance } = await WebAssembly.instantiate(bytes, {
  env: {
    print(ptr, len) {
      // Read the buffer per call: growing the memory detaches the old one.
      lines.push(decoder.decode(new Uint8Array(instance.exports.memory.buffer, ptr, len)));
    },
  },
});

const flush = () => process.stdout.write(lines.map((line) => line + "\n").join(""));

try {
  instance.exports.zhex_main();
  flush();
} catch (e) {
  flush();
  const code = instance.exports.zhex_last_error();
  if (code !== 0) {
    let message = failMessages[code] ?? "unknown fail code " + code;
    if (stagedFailCodes.has(code)) {
      const len = instance.exports.zhex_msg_len();
      message = decoder.decode(new Uint8Array(instance.exports.memory.buffer, 0, len));
    }
    process.stderr.write(`runtime error: ${message}\n`);
  } else if (/divide by zero|remainder by zero/.test(e.message)) {
    process.stderr.write("runtime error: division or remainder by zero\n");
  } else {
    // shortcut: any other native trap (failed cast, bounds, ...) prints verbatim; add matching above only if its text must be pinned
    process.stderr.write(`runtime error: ${e.message}\n`);
  }
  process.exit(1);
}
