# Word frequencies over the generated corpus, printing the top ten by
# count descending with word-ascending ties. The distinct-word array keeps
# first-seen order.
text = File.read("inputs/wordcount.txt")
counts = Hash.new(0)
distinct = []
text.split("\n").each do |line|
  line.split(" ").each do |w|
    next if w.empty?
    distinct << w unless counts.key?(w)
    counts[w] += 1
  end
end
entries = distinct.map { |w| [w, counts[w]] }
entries.sort! { |a, b| (b[1] <=> a[1]) == 0 ? a[0] <=> b[0] : b[1] <=> a[1] }
entries.first(10).each { |w, c| puts "#{w} #{c}" }
