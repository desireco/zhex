import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { runZhex } from '../src/scripts/zhex-run.mjs';
import { parseLifeFrames } from '../src/scripts/life-frames.mjs';
const encoder = new TextEncoder();
const data = JSON.parse(await readFile('data.json', 'utf8'));
const runnable = data.Examples.filter(e => e.Wasm);
// 22 = the corpus's 27 in-subset files minus the two library files (math.zhex,
// md.zhex) that appear only as Modules of their entry example, never as
// runnable entries, minus the three demos-page programs under examples/demos/
// that are not examples-page cards; twin pin of corpusTable in
// internal/wasm/corpus_test.go.
assert.equal(runnable.length, 22, 'runnable example count');
for (const example of data.Examples) {
  assert.ok(!example.Wasm || !example.WasmNote, `${example.ID} cannot be runnable and fenced at once`);
  if (!example.Wasm) assert(example.WasmNote, `${example.ID} fence note`);
}
// Astro copies public/wasm into _site/wasm verbatim, so exactly the runnable
// set plus the demos-page artifacts plus the solved AoC days' artifacts may
// exist there — a stale artifact from a removed or newly fenced example must
// fail the build instead of shipping dead bytes.
const aocDays = data.Aoc.filter(d => d.Wasm);
const onDisk = await readdir('../_site/wasm');
const expected = [...runnable.map(e => e.Wasm.replace('wasm/', '')), ...data.Demos.map(d => d.Wasm.replace('wasm/', '')), ...data.Aoc.filter(d => d.Wasm).map(d => d.Wasm.replace('wasm/', ''))];
assert.deepEqual([...onDisk].sort(), [...expected].sort(), '_site/wasm holds exactly the runnable, demo, and solved-door artifacts');
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
// #128: a part1 or solved door must be backed by a compiled artifact or
// carry the fence note, never neither and never both; an empty or working
// door carries neither, so its reveal keeps the status line. Every
// wasm-backed door's browser run over its sample stdin must reproduce the
// Go oracle's stdout byte for byte — the build-time differential gate.
for (const day of data.Aoc) {
  const solved = day.Status === 'part1' || day.Status === 'solved';
  assert.ok(!day.Wasm || !day.WasmNote, `aoc day ${day.Day} cannot be runnable and fenced at once`);
  if (solved) assert.ok(day.Wasm || day.WasmNote, `aoc day ${day.Day} panel backing`);
  else assert.ok(!day.Wasm && !day.WasmNote, `aoc day ${day.Day} keeps the status line`);
}
for (const day of data.Aoc.filter(d => d.Wasm)) {
  const bytes = await readFile(`../_site/${day.Wasm}`);
  assert(WebAssembly.validate(bytes), `aoc day ${day.Day} artifact validates`);
  const result = runZhex(bytes, { stdin: day.Stdin });
  assert.equal(result.error, null, `aoc day ${day.Day} run`);
  assert.deepEqual(result.stdout, encoder.encode(day.Output), `aoc day ${day.Day} stdout bytes equal the Go oracle`);
}
// #115's demos page replays exactly what parseLifeFrames extracts from each
// demo artifact's oracle stdout — the page runs the same parser over the
// bytes it captures in the browser, so these pins and the demos cannot drift
// apart. The reconstruction check inside parseLifeFrames is the differential:
// each demo's frames rebuild its oracle output byte-for-byte or the build
// fails, and the artifact run must match the oracle stdout bytes too. The
// pins derive from the parse — contiguous generations from 0, one square
// board, per-demo board/frame bounds, and a stdout size sanity cap (#116;
// the boards and run lengths are pinned individually — a wrong board or
// count for any demo fails the build). The examples page's 7×7 life
// card keeps its own pins through the general loop above.
assert.deepEqual(data.Demos.map(d => d.ID), ['gun', 'acorn', 'diehard'], 'demo order: glider gun, acorn, diehard');
for (const demo of data.Demos) assert.equal(demo.Group, 'Game of Life', `demo ${demo.ID} is grouped under Game of Life (#119)`);
const demoPins = {
  gun: { board: [54, 54], frames: [121, 121] },
  acorn: { board: [60, 60], frames: [151, 151] },
  diehard: { board: [24, 24], frames: [131, 131] },
};
for (const demo of data.Demos) {
  const pin = demoPins[demo.ID];
  assert(pin, `demo ${demo.ID} has verify pins`);
  assert(demo.Output.length < 600 * 1024, `demo ${demo.ID} stdout stays under ~600 KB`);
  const demoFrames = parseLifeFrames(demo.Output);
  assert.deepEqual(demoFrames.map(f => f.generation), demoFrames.map((_, i) => i), `demo ${demo.ID} generations run contiguously from 0`);
  const board = demoFrames[0].rows.length;
  assert(demoFrames.every(f => f.rows.length === board), `demo ${demo.ID} frames share one square board`);
  assert(board >= pin.board[0] && board <= pin.board[1], `demo ${demo.ID} board of ${board} stays within ${pin.board[0]}-${pin.board[1]} (≥12px cells)`);
  assert(demoFrames.length >= pin.frames[0] && demoFrames.length <= pin.frames[1], `demo ${demo.ID} runs ${demoFrames.length} frames, within ${pin.frames[0]}-${pin.frames[1]}`);
  const demoBytes = await readFile(`../_site/${demo.Wasm}`);
  assert(WebAssembly.validate(demoBytes), `demo ${demo.ID} artifact validates`);
  const demoRun = runZhex(demoBytes, { stdin: '' });
  assert.equal(demoRun.error, null, `demo ${demo.ID} artifact run`);
  assert.deepEqual(demoRun.stdout, encoder.encode(demo.Output), `demo ${demo.ID} artifact stdout bytes equal the build-time Go output`);
}
// The solved AoC days pin the same way the examples do (#129): each day's
// artifact runs on its committed sample input and must print the manifest's
// expected answers — the run panel's contract, proven at build time. A day
// that steps outside the compiled subset carries WasmNote instead of Wasm
// and is simply absent here.
for (const day of aocDays) {
  assert(!day.WasmNote, `aoc day ${day.Day} cannot be runnable and fenced at once`);
  assert.deepEqual(day.ExpectedAnswers.length, 2, `aoc day ${day.Day} pins two answers`);
  const bytes = await readFile(`../_site/${day.Wasm}`);
  assert(WebAssembly.validate(bytes), `aoc day ${day.Day} artifact validates`);
  const sample = await readFile(`../aoc/${day.Year}/${day.Sample}`);
  const result = runZhex(bytes, { stdin: sample });
  assert.equal(result.error, null, `aoc day ${day.Day} artifact run`);
  const answers = new TextDecoder().decode(result.stdout).trim().split('\n');
  assert.deepEqual(answers, day.ExpectedAnswers, `aoc day ${day.Day} prints the expected sample answers`);
}
