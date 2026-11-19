/* Build 200k numbered lines, then print the joined text's length and how
   many times "42" occurs across the lines (non-overlapping, per line).
   Each line is at most 61 bytes, so 200k lines fit a 13 MB buffer. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int count42(const char *s) {
    int hits = 0;
    const char *at = strstr(s, "42");
    while (at != NULL) {
        hits++;
        at = strstr(at + 2, "42");
    }
    return hits;
}

int main(void) {
    size_t cap = 200000 * 62 + 1;
    char *text = malloc(cap);
    if (text == NULL) return 1;
    size_t used = 0;
    long hits = 0;
    for (int i = 0; i < 200000; i++) {
        char line[80];
        int n = snprintf(line, sizeof line, "Line %d: the quick brown fox jumps over the lazy dog", i);
        hits += count42(line);
        if (i > 0) text[used++] = '\n';
        memcpy(text + used, line, n);
        used += n;
    }
    text[used] = '\0';
    printf("%zu\n%ld\n", strlen(text), hits);
    free(text);
    return 0;
}
