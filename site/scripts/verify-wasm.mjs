import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { runZhex } from '../src/scripts/zhex-run.mjs';
import { parseLifeFrames } from '../src/scripts/life-frames.mjs';
const encoder = new TextEncoder();
const data = JSON.parse(await readFile('data.json', 'utf8'));
const runnable = data.Examples.filter(e => e.Wasm);
// 22 = the corpus's 24 in-subset files minus the two library files (math.zhex,
// md.zhex) that appear only as Modules of their entry example, never as
// runnable entries; twin pin of corpusTable in internal/wasm/corpus_test.go.
assert.equal(runnable.length, 22, 'runnable example count');
for (const example of data.Examples) {
  assert.ok(!example.Wasm || !example.WasmNote, `${example.ID} cannot be runnable and fenced at once`);
  if (!example.Wasm) assert(example.WasmNote, `${example.ID} fence note`);
}
// Astro copies public/wasm into _site/wasm verbatim, so exactly the runnable
// set plus the demos-page artifacts may exist there — a stale artifact from a
// removed or newly fenced example must fail the build instead of shipping
// dead bytes.
const onDisk = await readdir('../_site/wasm');
const expected = [...runnable.map(e => e.Wasm.replace('wasm/', '')), ...data.Demos.map(d => d.Wasm.replace('wasm/', ''))];
assert.deepEqual([...onDisk].sort(), [...expected].sort(), '_site/wasm holds exactly the runnable and demo artifacts');
for (const example of runnable) {
  const bytes = await readFile(`../_site/${example.Wasm}`);
  assert(WebAssembly.validate(bytes), `${example.ID} artifact validates`);
  const result = runZhex(bytes, { stdin: example.Stdin });
  assert.equal(result.error, null, `${example.ID} run`);
  assert.deepEqual(result.stdout, encoder.encode(example.Output), `${example.ID} stdout bytes equal the build-time Go output`);
}
// Edited-input + clean state: every runZhex call compiles a fresh instance,
// so a stdin edit cannot leak into a later default-input run of the same
// artifact — the last assert re-runs factorial with its oracle stdin after
// both edited runs and must match the oracle byte-for-byte again.
const byId = Object.fromEntries(data.Examples.map(e => [e.ID, e]));
const factorialBytes = await readFile(`../_site/${byId.factorial.Wasm}`);
const edited = runZhex(factorialBytes, { stdin: '10\n' });
assert.equal(edited.error, null, 'factorial edited-input run');
assert(new TextDecoder().decode(edited.stdout).includes('3628800'), 'factorial prints 10! under edited stdin');
const helloBytes = await readFile(`../_site/${byId.hello.Wasm}`);
const greeted = runZhex(helloBytes, { stdin: 'Grace\n' });
assert.equal(greeted.error, null, 'hello edited-input run');
assert.deepEqual(greeted.stdout, encoder.encode('Your name: Hello, Grace!\n'), 'hello greets the edited name');
const rerun = runZhex(factorialBytes, { stdin: byId.factorial.Stdin });
assert.equal(rerun.error, null, 'factorial default re-run');
assert.deepEqual(rerun.stdout, encoder.encode(byId.factorial.Output), 'factorial default re-run matches the oracle after the edit (fresh-instance proof)');
// #115's demos page replays exactly what parseLifeFrames extracts from each
// demo artifact's oracle stdout — the page runs the same parser over the
// bytes it captures in the browser, so this pin and the demo cannot drift
// apart. The reconstruction check inside parseLifeFrames is the differential:
// these frames rebuild the oracle output byte-for-byte or the build fails.
// The pins derive from the parse — contiguous generations from 0, one square
// board in the size range the canvas renders at ≥12px cells, a run long
// enough to be worth a loop — never a hard-coded frame list. The examples
// page's 7×7 life card keeps its own pins through the general loop above.
assert.equal(data.Demos.length, 1, 'demo manifest holds one demo');
const demoFrames = parseLifeFrames(data.Demos[0].Output);
assert.deepEqual(demoFrames.map(f => f.generation), demoFrames.map((_, i) => i), 'demo generations run contiguously from 0');
const board = demoFrames[0].rows.length;
assert(demoFrames.every(f => f.rows.length === board), 'demo frames share one square board');
assert(board >= 21 && board <= 30, 'demo board stays in the 21-30 range (336px stage, ≥12px cells)');
assert(demoFrames.length >= 49 && demoFrames.length <= 61, 'demo runs 48-60 computed generations');
