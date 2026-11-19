# Build 200k numbered lines, then print the joined text's length and how
# many times "42" occurs across the lines (non-overlapping, per line).
hits = 0
lines = []
for i in range(200000):
    line = "Line " + str(i) + ": the quick brown fox jumps over the lazy dog"
    hits += line.count("42")
    lines.append(line)
text = "\n".join(lines)
print(len(text))
print(hits)
