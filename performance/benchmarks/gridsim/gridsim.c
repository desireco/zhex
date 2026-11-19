/* Heat-style double-buffer diffusion on a 400x400 grid for 10 generations.
   Cells start at (r*131 + c*137) % 100; borders stay fixed; interior cells
   become (4*cur + four neighbours) / 8. Prints the final cell sum. */
#include <stdio.h>

#define SIZE 400
#define GENERATIONS 10

static long long a[SIZE][SIZE], b[SIZE][SIZE];

int main(void) {
    long long (*current)[SIZE] = a, (*following)[SIZE] = b;
    for (int r = 0; r < SIZE; r++)
        for (int c = 0; c < SIZE; c++)
            current[r][c] = (r * 131 + c * 137) % 100;
    for (int g = 0; g < GENERATIONS; g++) {
        for (int r = 0; r < SIZE; r++) {
            for (int c = 0; c < SIZE; c++) {
                if (r == 0 || r == SIZE - 1 || c == 0 || c == SIZE - 1)
                    following[r][c] = current[r][c];
                else
                    following[r][c] = (current[r][c] * 4 + current[r - 1][c] + current[r + 1][c] + current[r][c - 1] + current[r][c + 1]) / 8;
            }
        }
        long long (*swap)[SIZE] = current;
        current = following;
        following = swap;
    }
    long long total = 0;
    for (int r = 0; r < SIZE; r++)
        for (int c = 0; c < SIZE; c++)
            total += current[r][c];
    printf("%lld\n", total);
    return 0;
}
