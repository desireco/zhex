<?php
// Word frequencies over the generated corpus, printing the top ten by
// count descending with word-ascending ties. The distinct-word array keeps
// first-seen order.
$text = file_get_contents("inputs/wordcount.txt");
$counts = [];
$distinct = [];
foreach (explode("\n", $text) as $line) {
    foreach (explode(" ", $line) as $w) {
        if ($w === "") continue;
        if (!array_key_exists($w, $counts)) {
            $distinct[] = $w;
            $counts[$w] = 0;
        }
        $counts[$w]++;
    }
}
usort($distinct, function ($a, $b) use ($counts) {
    if ($counts[$b] !== $counts[$a]) return $counts[$b] <=> $counts[$a];
    return strcmp($a, $b);
});
foreach (array_slice($distinct, 0, 10) as $w) {
    echo $w, " ", $counts[$w], "\n";
}
