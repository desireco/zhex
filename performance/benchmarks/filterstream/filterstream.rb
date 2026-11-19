# Print stdin lines containing "ERROR", streaming until end of input.
while (line = gets)
  line = line.chomp
  puts line if line.include?("ERROR")
end
