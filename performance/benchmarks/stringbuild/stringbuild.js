// Build 200k numbered lines, then print the joined text's length and how
// many times "42" occurs across the lines (non-overlapping, per line). Node and Bun.
const lines = [];
let hits = 0;
for (let i = 0; i < 200000; i++) {
    const line = "Line " + i + ": the quick brown fox jumps over the lazy dog";
    hits += line.split("42").length - 1;
    lines.push(line);
}
const text = lines.join("\n");
console.log(text.length);
console.log(hits);
