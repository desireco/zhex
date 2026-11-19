/* Print stdin lines containing "ERROR", streaming until end of input. */
#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main(void) {
    char *line = NULL;
    size_t cap = 0;
    ssize_t n;
    while ((n = getline(&line, &cap, stdin)) != -1) {
        if (memmem(line, n, "ERROR", 5) != NULL)
            fputs(line, stdout);
    }
    free(line);
    return 0;
}
