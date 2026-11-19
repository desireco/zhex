<?php
// Build 200k numbered lines, then print the joined text's length and how
// many times "42" occurs across the lines (non-overlapping, per line).
$hits = 0;
$lines = [];
for ($i = 0; $i < 200000; $i++) {
    $line = "Line " . $i . ": the quick brown fox jumps over the lazy dog";
    $hits += substr_count($line, "42");
    $lines[] = $line;
}
$text = implode("\n", $lines);
echo strlen($text), "\n", $hits, "\n";
