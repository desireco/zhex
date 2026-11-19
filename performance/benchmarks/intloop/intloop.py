# Euclid's gcd by remainder over 100k pairs from an in-program Park-Miller
# generator (state * 48271 % 2147483647, seeded 20260119); prints the gcd sum.
def gcd(a, b):
    x, y = a, b
    while y != 0:
        x, y = y, x % y
    return x


state = 20260119
total = 0
for _ in range(100000):
    state = state * 48271 % 2147483647
    a = state % 999983 + 1
    state = state * 48271 % 2147483647
    b = state % 999979 + 1
    total += gcd(a, b)
print(total)
