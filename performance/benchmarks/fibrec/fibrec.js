// Naive recursive Fibonacci at 30 — the call-recursion shape. Runs under Node and Bun.
function fib(n) {
    if (n < 2) return n;
    return fib(n - 1) + fib(n - 2);
}
console.log(fib(30));
