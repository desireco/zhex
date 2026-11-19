# Euclid's gcd by remainder over 100k pairs from an in-program Park-Miller
# generator (state * 48271 % 2147483647, seeded 20260119); prints the gcd sum.
def gcd(a, b)
  x, y = a, b
  while y != 0
    rest = y
    y = x % y
    x = rest
  end
  x
end

state = 20260119
total = 0
100_000.times do
  state = state * 48271 % 2147483647
  a = state % 999_983 + 1
  state = state * 48271 % 2147483647
  b = state % 999_979 + 1
  total += gcd(a, b)
end
puts total
