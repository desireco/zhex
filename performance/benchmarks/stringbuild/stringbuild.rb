# Build 200k numbered lines, then print the joined text's length and how
# many times "42" occurs across the lines (non-overlapping, per line).
hits = 0
lines = []
200_000.times do |i|
  line = "Line #{i}: the quick brown fox jumps over the lazy dog"
  hits += line.scan("42").size
  lines << line
end
text = lines.join("\n")
puts text.size
puts hits
