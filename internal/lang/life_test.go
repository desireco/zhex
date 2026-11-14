package lang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestLifeBlockBlinkerAndGliderOracles(t *testing.T) {
	source, err := os.ReadFile("../../examples/life.zhex")
	if err != nil {
		t.Fatal(err)
	}
	oracles := `
let block: Array<Int> = array.new([7, 7], 0)
block[2, 2] = 1
block[2, 3] = 1
block[3, 2] = 1
block[3, 3] = 1
let block-next = advance(block)
print("BLOCK")
show(block-next, 1)

let blinker: Array<Int> = array.new([7, 7], 0)
blinker[3, 2] = 1
blinker[3, 3] = 1
blinker[3, 4] = 1
let blinker-vertical = advance(blinker)
let blinker-horizontal = advance(blinker-vertical)
print("BLINKER 1")
show(blinker-vertical, 1)
print("BLINKER 2")
show(blinker-horizontal, 2)

let edge: Array<Int> = array.new([7, 7], 0)
edge[3, 0] = 1
edge[3, 1] = 1
edge[3, 2] = 1
let edge-next = advance(edge)
print("EDGE")
show(edge-next, 1)
`
	var output bytes.Buffer
	if err := Run("life-oracle.zhex", string(source)+oracles, bytes.NewBuffer(nil), &output); err != nil {
		t.Fatal(err)
	}
	const want = `Generation 0
.......
.......
...#...
....#..
..###..
.......
.......
Generation 1
.......
.......
.......
..#.#..
...##..
...#...
.......
Generation 2
.......
.......
.......
....#..
..#.#..
...##..
.......
Generation 3
.......
.......
.......
...#...
....##.
...##..
.......
Generation 4
.......
.......
.......
....#..
.....#.
...###.
.......
BLOCK
Generation 1
.......
.......
..##...
..##...
.......
.......
.......
BLINKER 1
Generation 1
.......
.......
...#...
...#...
...#...
.......
.......
BLINKER 2
Generation 2
.......
.......
.......
..###..
.......
.......
.......
EDGE
Generation 1
.......
.......
.#.....
.#.....
.#.....
.......
.......
`
	if output.String() != want {
		t.Fatalf("Life patterns differ from their oracles:\n got:\n%s\nwant:\n%s", output.String(), want)
	}
}

// Generations must advance the same board once per requested step.
func TestLifeGenerationCountsAndTrace(t *testing.T) {
	source, err := os.ReadFile("../../examples/life.zhex")
	if err != nil {
		t.Fatal(err)
	}
	var full bytes.Buffer
	if err := Run("life.zhex", string(source), strings.NewReader(""), &full); err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(full.String(), "\n")
	for _, count := range []int{0, 1, 4} {
		program := strings.Replace(string(source), "generations(4)", fmt.Sprintf("generations(%d)", count), 1)
		var out, trace bytes.Buffer
		if err := RunWithTrace("life.zhex", program, strings.NewReader(""), &out, &trace); err != nil {
			t.Fatal(err)
		}
		want := strings.Join(lines[:(count+1)*8], "")
		if out.String() != want {
			t.Errorf("%d generations: got %q, want %q", count, out.String(), want)
		}
		calls := map[string]int{}
		for _, line := range strings.Split(strings.TrimSpace(trace.String()), "\n") {
			var event struct {
				Event string `json:"event"`
				Word  string `json:"word"`
			}
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				t.Fatal(err)
			}
			if event.Event == "word-call" {
				calls[event.Word]++
			}
		}
		if calls["seed"] != 1 || calls["generations"] != 1 || calls["advance"] != count || calls["show"] != count+1 {
			t.Errorf("%d generations: unexpected calls %v", count, calls)
		}
	}
}
