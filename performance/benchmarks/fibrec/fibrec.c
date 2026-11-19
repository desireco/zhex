/* Naive recursive Fibonacci at 30 — the call-recursion shape. */
#include <stdio.h>

long long fib(long long n) {
    if (n < 2) return n;
    return fib(n - 1) + fib(n - 2);
}

int main(void) {
    printf("%lld\n", fib(30));
    return 0;
}
