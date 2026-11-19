<?php
// Print stdin lines containing "ERROR", streaming until end of input.
while (($line = fgets(STDIN)) !== false) {
    if (strpos($line, "ERROR") !== false) {
        echo $line;
    }
}
