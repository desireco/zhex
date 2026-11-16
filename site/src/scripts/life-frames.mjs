// Shared Game of Life frame parser (#90): the examples-page demo and the site
// build's verify-wasm differential must agree on exactly what the life.wasm
// stdout holds. Splits the captured stdout on its "Generation N" header lines,
// validates each frame as a 7×7 board over [.#], and refuses any output the
// parsed frames cannot byte-reconstruct — that reconstruction is the
// differential check, run against the build-time oracle here and against the
// in-browser runZhex capture on the page.
export function parseLifeFrames(stdout) {
  const lines = stdout.split("\n");
  if (lines.pop() !== "") throw new Error("life output must end with a newline");
  const frames = [];
  let at = 0;
  while (at < lines.length) {
    const header = /^Generation ([0-9]+)$/.exec(lines[at]);
    if (header === null) throw new Error(`expected a "Generation N" header, found ${JSON.stringify(lines[at])}`);
    const rows = lines.slice(at + 1, at + 8);
    if (rows.length !== 7 || rows.some(row => !/^[.#]{7}$/.test(row))) {
      throw new Error(`generation ${header[1]} is not 7 rows of 7 [.#] characters`);
    }
    frames.push({ generation: Number(header[1]), rows });
    at += 8;
  }
  if (frames.length === 0) throw new Error("life stdout holds no frames");
  const rebuilt = frames.map(frame => `Generation ${frame.generation}\n${frame.rows.join("\n")}\n`).join("");
  if (rebuilt !== stdout) throw new Error("parsed frames do not byte-reconstruct the artifact output");
  return frames;
}
