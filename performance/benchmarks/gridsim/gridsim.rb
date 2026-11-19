# Heat-style double-buffer diffusion on a 400x400 grid for 10 generations.
# Cells start at (r*131 + c*137) % 100; borders stay fixed; interior cells
# become (4*cur + four neighbours) / 8. Prints the final cell sum.
SIZE = 400
GENERATIONS = 10

def make_grid
  Array.new(SIZE) { Array.new(SIZE, 0) }
end

current = make_grid
following = make_grid
SIZE.times do |r|
  SIZE.times do |c|
    current[r][c] = (r * 131 + c * 137) % 100
  end
end
GENERATIONS.times do
  SIZE.times do |r|
    SIZE.times do |c|
      if r == 0 || r == SIZE - 1 || c == 0 || c == SIZE - 1
        following[r][c] = current[r][c]
      else
        following[r][c] = (current[r][c] * 4 + current[r - 1][c] + current[r + 1][c] + current[r][c - 1] + current[r][c + 1]) / 8
      end
    end
  end
  current, following = following, current
end
total = 0
SIZE.times do |r|
  SIZE.times do |c|
    total += current[r][c]
  end
end
puts total
