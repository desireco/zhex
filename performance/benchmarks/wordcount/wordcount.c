/* Word frequencies over the generated corpus, printing the top ten by
   count descending with word-ascending ties. The distinct-word array keeps
   first-seen order. Hashing is a plain open-addressing FNV-1a table — the
   ordinary C hand-rolled frequency count. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define TABLE_SIZE (1 << 17)

typedef struct {
    const char *word;
    long count;
} slot;

static slot table[TABLE_SIZE];

static unsigned long fnv1a(const char *s) {
    unsigned long h = 1469598103934665603UL;
    while (*s) {
        h ^= (unsigned char)*s++;
        h *= 1099511628211UL;
    }
    return h;
}

static int cmp(const void *pa, const void *pb) {
    const slot *a = pa, *b = pb;
    if (a->count != b->count) return (a->count > b->count) ? -1 : 1;
    return strcmp(a->word, b->word);
}

int main(void) {
    FILE *f = fopen("inputs/wordcount.txt", "r");
    if (!f) return 1;
    fseek(f, 0, SEEK_END);
    long size = ftell(f);
    fseek(f, 0, SEEK_SET);
    char *data = malloc(size + 1);
    if (!f || fread(data, 1, size, f) != (size_t)size) return 1;
    fclose(f);
    data[size] = '\0';

    size_t distinct_cap = 4096, distinct_len = 0;
    char **distinct = malloc(distinct_cap * sizeof *distinct);
    char *w = strtok(data, " \n");
    while (w) {
        unsigned long h = fnv1a(w) & (TABLE_SIZE - 1);
        while (table[h].word && strcmp(table[h].word, w) != 0)
            h = (h + 1) & (TABLE_SIZE - 1);
        if (!table[h].word) {
            if (distinct_len == distinct_cap) {
                distinct_cap *= 2;
                distinct = realloc(distinct, distinct_cap * sizeof *distinct);
            }
            table[h].word = w;
            distinct[distinct_len++] = w;
        }
        table[h].count++;
        w = strtok(NULL, " \n");
    }

    slot *entries = malloc(distinct_len * sizeof *entries);
    for (size_t i = 0; i < distinct_len; i++) {
        unsigned long h = fnv1a(distinct[i]) & (TABLE_SIZE - 1);
        while (strcmp(table[h].word, distinct[i]) != 0)
            h = (h + 1) & (TABLE_SIZE - 1);
        entries[i] = table[h];
    }
    qsort(entries, distinct_len, sizeof *entries, cmp);
    for (int i = 0; i < 10; i++)
        printf("%s %ld\n", entries[i].word, entries[i].count);
    return 0;
}
