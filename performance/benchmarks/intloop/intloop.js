// Euclid's gcd by remainder over 100k pairs from an in-program Park-Miller
// generator (state * 48271 % 2147483647, seeded 20260119); prints the gcd sum.
// The generator stays under 2^53 so double arithmetic is exact. Node and Bun.
function gcd(a, b) {
    let x = a, y = b;
    while (y !== 0) {
        const rest = y;
        y = x % y;
        x = rest;
    }
    return x;
}

let state = 20260119;
let total = 0;
for (let i = 0; i < 100000; i++) {
    state = (state * 48271) % 2147483647;
    const a = (state % 999983) + 1;
    state = (state * 48271) % 2147483647;
    const b = (state % 999979) + 1;
    total += gcd(a, b);
}
console.log(total);
