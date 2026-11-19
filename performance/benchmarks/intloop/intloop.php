<?php
// Euclid's gcd by remainder over 100k pairs from an in-program Park-Miller
// generator (state * 48271 % 2147483647, seeded 20260119); prints the gcd sum.
function gcd($a, $b) {
    $x = $a; $y = $b;
    while ($y != 0) {
        $rest = $y;
        $y = $x % $y;
        $x = $rest;
    }
    return $x;
}

$state = 20260119;
$total = 0;
for ($i = 0; $i < 100000; $i++) {
    $state = $state * 48271 % 2147483647;
    $a = $state % 999983 + 1;
    $state = $state * 48271 % 2147483647;
    $b = $state % 999979 + 1;
    $total += gcd($a, $b);
}
echo $total, "\n";
