# Print stdin lines containing "ERROR", streaming until end of input.
import sys

for line in sys.stdin:
    if "ERROR" in line:
        sys.stdout.write(line)
