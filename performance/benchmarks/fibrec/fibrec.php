<?php
// Naive recursive Fibonacci at 30 — the call-recursion shape.
function fib($n) {
    if ($n < 2) return $n;
    return fib($n - 1) + fib($n - 2);
}
echo fib(30), "\n";
