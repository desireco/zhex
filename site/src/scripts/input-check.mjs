// The #107 input-box validation, shared by the examples page and the
// playground (#88): the box is checked against its declared kind before any
// run, so a wrong entry is named here instead of surfacing as the program's
// own trap. Integer lines mirror text.to-int: trimmed, then a signed 128-bit
// decimal integer; "text" and "none" accept anything.

const INT128_MIN = -(2n ** 127n);
const INT128_MAX = 2n ** 127n - 1n;

function isInt128(text) {
  if (!/^[+-]?[0-9]+$/.test(text)) return false;
  const value = BigInt(text);
  return value >= INT128_MIN && value <= INT128_MAX;
}

export function inputProblem(kind, lines, value) {
  if (kind !== "int") return "";
  const phrases = { 1: "one number", 2: "two numbers, one per line" };
  const expected = phrases[lines] ?? `${lines} numbers, one per line`;
  const raw = value.replace(/\n$/, "");
  const entered = raw === "" ? [] : raw.split("\n");
  if (entered.length !== lines) {
    return entered.length === 0
      ? `Expected ${expected} — the box is empty.`
      : `Expected ${expected} — ${entered.length} ${entered.length === 1 ? "line" : "lines"} found.`;
  }
  const bad = entered.find(line => !isInt128(line.trim()));
  if (bad === undefined) return "";
  return bad.trim() === ""
    ? `Expected ${expected} — a blank line is not an integer.`
    : `Expected ${expected} — "${bad.trim()}" is not an integer.`;
}
