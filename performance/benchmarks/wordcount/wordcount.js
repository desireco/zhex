// Word frequencies over the generated corpus, printing the top ten by
// count descending with word-ascending ties. The distinct-word array keeps
// first-seen order. Node and Bun.
const fs = require("fs");
const text = fs.readFileSync("inputs/wordcount.txt", "utf8");
const counts = new Map();
const distinct = [];
for (const line of text.split("\n")) {
    for (const w of line.split(" ")) {
        if (w.length === 0) continue;
        if (!counts.has(w)) {
            distinct.push(w);
            counts.set(w, 0);
        }
        counts.set(w, counts.get(w) + 1);
    }
}
const entries = distinct.map((w) => ({ word: w, count: counts.get(w) }));
entries.sort((a, b) => b.count - a.count || (a.word < b.word ? -1 : 1));
for (const e of entries.slice(0, 10)) {
    console.log(e.word + " " + e.count);
}
