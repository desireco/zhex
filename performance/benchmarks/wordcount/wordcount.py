# Word frequencies over the generated corpus, printing the top ten by
# count descending with word-ascending ties. The distinct-word list keeps
# first-seen order.
text = open("inputs/wordcount.txt", encoding="utf-8").read()
counts = {}
distinct = []
for line in text.split("\n"):
    for w in line.split(" "):
        if not w:
            continue
        if w not in counts:
            distinct.append(w)
            counts[w] = 0
        counts[w] += 1
entries = sorted(distinct, key=lambda w: (-counts[w], w))
for w in entries[:10]:
    print(w, counts[w])
