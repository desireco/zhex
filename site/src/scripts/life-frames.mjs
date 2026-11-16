// Shared Game of Life frame parser (#90, generalized in #115): the demos-page
// player and the site build's verify-wasm differential must agree on exactly
// what a demo artifact's stdout holds. Splits the captured stdout on its
// "Generation N" header lines, validates every frame as a square board over
// [.#] with one shared size across the run, and refuses any output the parsed
// frames cannot byte-reconstruct — that reconstruction is the differential
// check, run against the build-time oracle here and against the in-browser
// runZhex capture on the page.
export function parseLifeFrames(stdout) {
  const lines = stdout.split("\n");
  if (lines.pop() !== "") throw new Error("life output must end with a newline");
  const frames = [];
  let board = 0;
  let at = 0;
  while (at < lines.length) {
    const header = /^Generation ([0-9]+)$/.exec(lines[at]);
    if (header === null) throw new Error(`expected a "Generation N" header, found ${JSON.stringify(lines[at])}`);
    let end = at + 1;
    while (end < lines.length && !/^Generation [0-9]+$/.test(lines[end])) end += 1;
    const rows = lines.slice(at + 1, end);
    const size = rows.length;
    if (size === 0 || rows.some(row => !new RegExp(`^[.#]{${size}}$`).test(row))) {
      throw new Error(`generation ${header[1]} is not ${size} rows of ${size} [.#] characters`);
    }
    if (board === 0) board = size;
    else if (size !== board) throw new Error(`generation ${header[1]} is ${size} rows; frame 0 is ${board}`);
    frames.push({ generation: Number(header[1]), rows });
    at = end;
  }
  if (frames.length === 0) throw new Error("life stdout holds no frames");
  const rebuilt = frames.map(frame => `Generation ${frame.generation}\n${frame.rows.join("\n")}\n`).join("");
  if (rebuilt !== stdout) throw new Error("parsed frames do not byte-reconstruct the artifact output");
  return frames;
}
