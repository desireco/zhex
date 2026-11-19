<?php
// Heat-style double-buffer diffusion on a 400x400 grid for 10 generations.
// Cells start at (r*131 + c*137) % 100; borders stay fixed; interior cells
// become (4*cur + four neighbours) / 8. Prints the final cell sum.
const SIZE = 400;
const GENERATIONS = 10;

function make_grid() {
    return array_fill(0, SIZE, array_fill(0, SIZE, 0));
}

$current = make_grid();
$following = make_grid();
for ($r = 0; $r < SIZE; $r++)
    for ($c = 0; $c < SIZE; $c++)
        $current[$r][$c] = ($r * 131 + $c * 137) % 100;
for ($g = 0; $g < GENERATIONS; $g++) {
    for ($r = 0; $r < SIZE; $r++) {
        for ($c = 0; $c < SIZE; $c++) {
            if ($r == 0 || $r == SIZE - 1 || $c == 0 || $c == SIZE - 1)
                $following[$r][$c] = $current[$r][$c];
            else
                $following[$r][$c] = intdiv($current[$r][$c] * 4 + $current[$r - 1][$c] + $current[$r + 1][$c] + $current[$r][$c - 1] + $current[$r][$c + 1], 8);
        }
    }
    $swap = $current; $current = $following; $following = $swap;
}
$total = 0;
for ($r = 0; $r < SIZE; $r++)
    for ($c = 0; $c < SIZE; $c++)
        $total += $current[$r][$c];
echo $total, "\n";
