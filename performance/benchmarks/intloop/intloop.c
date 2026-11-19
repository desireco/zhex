/* Euclid's gcd by remainder over 100k pairs from an in-program Park-Miller
   generator (state * 48271 % 2147483647, seeded 20260119); prints the gcd sum. */
#include <stdio.h>

long long gcd(long long a, long long b) {
    long long x = a, y = b;
    while (y != 0) {
        long long rest = y;
        y = x % y;
        x = rest;
    }
    return x;
}

int main(void) {
    long long state = 20260119, total = 0;
    for (int i = 0; i < 100000; i++) {
        state = state * 48271 % 2147483647;
        long long a = state % 999983 + 1;
        state = state * 48271 % 2147483647;
        long long b = state % 999979 + 1;
        total += gcd(a, b);
    }
    printf("%lld\n", total);
    return 0;
}
