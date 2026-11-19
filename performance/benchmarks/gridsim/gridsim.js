// Heat-style double-buffer diffusion on a 400x400 grid for 10 generations.
// Cells start at (r*131 + c*137) % 100; borders stay fixed; interior cells
// become (4*cur + four neighbours) / 8. Prints the final cell sum. Node and Bun.
const SIZE = 400;
const GENERATIONS = 10;
const make = () => Array.from({ length: SIZE }, () => new Array(SIZE).fill(0));
let current = make();
let following = make();
for (let r = 0; r < SIZE; r++)
    for (let c = 0; c < SIZE; c++)
        current[r][c] = (r * 131 + c * 137) % 100;
for (let g = 0; g < GENERATIONS; g++) {
    for (let r = 0; r < SIZE; r++) {
        for (let c = 0; c < SIZE; c++) {
            if (r === 0 || r === SIZE - 1 || c === 0 || c === SIZE - 1)
                following[r][c] = current[r][c];
            else
                following[r][c] = Math.trunc((current[r][c] * 4 + current[r - 1][c] + current[r + 1][c] + current[r][c - 1] + current[r][c + 1]) / 8);
        }
    }
    [current, following] = [following, current];
}
let total = 0;
for (let r = 0; r < SIZE; r++)
    for (let c = 0; c < SIZE; c++)
        total += current[r][c];
console.log(total);
