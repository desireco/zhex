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
// set may exist there — a stale artifact from a removed or newly fenced
// example must fail the build instead of shipping dead bytes.
const onDisk = await readdir('../_site/wasm');
const expected = runnable.map(e => e.Wasm.replace('wasm/', ''));
assert.deepEqual([...onDisk].sort(), [...expected].sort(), '_site/wasm holds exactly the runnable artifacts');
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
// #90's demo replays exactly what parseLifeFrames extracts from the artifact's
// stdout — the page runs the same parser over the bytes it captures in the
// browser, so this pin and the demo cannot drift apart. The reconstruction
// check inside parseLifeFrames is the differential: these frames rebuild the
// oracle output byte-for-byte or the build fails.
const lifeFrames = parseLifeFrames(byId.life.Output);
assert.equal(lifeFrames.length, 5, 'life stdout holds exactly five frames');
assert.equal(lifeFrames[0].generation, 0, 'life frames start at generation 0');
assert.deepEqual(lifeFrames.map(f => f.generation), [0, 1, 2, 3, 4], 'life generations run 0 through 4');
