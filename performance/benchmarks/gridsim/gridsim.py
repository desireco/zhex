# Heat-style double-buffer diffusion on a 400x400 grid for 10 generations.
# Cells start at (r*131 + c*137) % 100; borders stay fixed; interior cells
# become (4*cur + four neighbours) / 8. Prints the final cell sum.
SIZE = 400
GENERATIONS = 10

current = [[(r * 131 + c * 137) % 100 for c in range(SIZE)] for r in range(SIZE)]
following = [[0] * SIZE for _ in range(SIZE)]
for _ in range(GENERATIONS):
    for r in range(SIZE):
        for c in range(SIZE):
            if r == 0 or r == SIZE - 1 or c == 0 or c == SIZE - 1:
                following[r][c] = current[r][c]
            else:
                following[r][c] = (current[r][c] * 4 + current[r - 1][c] + current[r + 1][c] + current[r][c - 1] + current[r][c + 1]) // 8
    current, following = following, current
total = sum(sum(row) for row in current)
print(total)
