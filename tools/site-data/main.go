package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"zhex/internal/lang"
	"zhex/internal/wasm"
)

type Example struct {
	ID, Title, Description string
	Group                  string
	Source, Input, Output  string
	Highlighted            string
	Command, SourceURL     string
	// Stdin is the raw manifest input, trailing newline included; Input is
	// the trimmed display value.
	Stdin string
	// InputKind names what the run panel's box expects — "none" (the
	// program reads nothing), "int" (InputLines integer lines), or "text"
	// (InputLines lines of text) — and InputHint is the short human
	// description rendered under the Program input label.
	InputKind  string
	InputLines int
	InputHint  string
	// Wasm is the compiled artifact's relative URL when the example is in
	// the compiled subset, and WasmNote the fence diagnostic message when it
	// is not; the -wasm pass sets them.
	Wasm, WasmNote string
	// File labels the entry source's code block; Modules stacks labeled
	// library files under it when the example spans several files.
	File    string
	Modules []ExampleModule
}

// ExampleModule is one labeled secondary source beside the entry file.
type ExampleModule struct {
	File, Source, URL string
	Highlighted       string
}

// Demo is one demos-page program: the demo is not an examples-page card, so
// it carries no source or run panel — the demos page replays Output's frames
// from the compiled artifact client-side. Title and Blurb head the player;
// Stdin is always empty today; the field travels with the data so a future
// demo can read input.
type Demo struct {
	ID     string
	Group  string
	Title  string
	Blurb  string
	Wasm   string
	Stdin  string
	Output string
}

// AocDay is one Advent of Code day on the AoC page (#126): a run panel
// mounts only on a part1 or solved day, backed by Wasm with the Go oracle's
// sample stdout in Output (#128) — or, for an out-of-subset solution, by the
// WasmNote fence diagnostic naming the construct.
type AocDay struct {
	Year            int
	Day             int
	Title           string
	Outline         string
	OfficialURL     string
	Status          string // empty | working | part1 | solved
	Sample          string // sample input path, relative to aoc/<year>/
	ExpectedAnswers []string
	// Stdin is the sample's raw contents — the run panel's prefill and the
	// oracle run's stdin — empty until the day has a sample.
	Stdin string
	// Output is the Go oracle's sample stdout for a part1 or solved day; the
	// build gate compares the browser run's bytes against it.
	Output string
	// Wasm is the compiled artifact's relative URL and WasmNote the fence
	// diagnostic message, matching Example's fields; both stay empty until
	// a day compiles.
	Wasm, WasmNote string
	// InputKind names what the day's run panel input box expects, matching
	// Example's kind vocabulary.
	InputKind string
}

type pageData struct {
	Manifesto  Manifesto
	HeroSource string
	HeroInput  string
	HeroOutput string
	Examples   []Example
	Demos      []Demo
	Aoc        []AocDay
	Roadmap    Roadmap
	Catalog    []CatalogEntry
	Nav        template.HTML
}

// CatalogEntry is one stable diagnostic code with its catalog prose; the
// source of truth is the embedded catalog in internal/lang.
type CatalogEntry struct {
	Code, Text string
}

// loadCatalog splits the language's catalog markdown into one entry per
// "## code" section, skipping the intro that precedes the first section.
func loadCatalog() ([]CatalogEntry, error) {
	sections := strings.Split(strings.TrimSpace(lang.CatalogMarkdown()), "\n## ")
	var entries []CatalogEntry
	for i, section := range sections {
		if i == 0 {
			continue
		}
		code, text, _ := strings.Cut(section, "\n")
		entries = append(entries, CatalogEntry{strings.TrimSpace(code), strings.TrimSpace(text)})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("diagnostic catalog has no sections")
	}
	return entries, nil
}

type Roadmap struct {
	Intro    string
	Sections []RoadmapSection
	// DoneGroups renders the Done section for the changelog page: the
	// grouping is mechanical, derived from each bullet's leading prefix.
	DoneGroups []RoadmapGroup
}

type RoadmapSection struct {
	Title, Summary string
	Items          []string
}

// RoadmapGroup is one changelog group: every Done bullet whose leading
// "Name:" prefix shares the name, in first-appearance order, with the
// section's own items untouched underneath.
type RoadmapGroup struct {
	Name, Anchor string
	Count        int
	Items        []string
}

// The roadmap uses plain paragraphs, level-two headings, and single-line bullets.
// Keep this small format explicit rather than silently dropping Markdown syntax.
func loadRoadmap(source []byte) (Roadmap, error) {
	var roadmap Roadmap
	lines := strings.Split(strings.TrimSpace(string(source)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "# Zhex roadmap" {
		return roadmap, fmt.Errorf("ROADMAP.md must start with # Zhex roadmap")
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			roadmap.Sections = append(roadmap.Sections, RoadmapSection{Title: strings.TrimPrefix(line, "## ")})
			continue
		}
		if strings.HasPrefix(line, "#") || (strings.ContainsAny(line, "`*_[]<>") && !strings.HasPrefix(line, "- ")) {
			return roadmap, fmt.Errorf("unsupported roadmap markup: %s", line)
		}
		if len(roadmap.Sections) == 0 {
			if strings.HasPrefix(line, "- ") {
				return roadmap, fmt.Errorf("roadmap bullet outside a section: %s", line)
			}
			roadmap.Intro = strings.TrimSpace(roadmap.Intro + " " + line)
			continue
		}
		section := &roadmap.Sections[len(roadmap.Sections)-1]
		if strings.HasPrefix(line, "- ") {
			section.Items = append(section.Items, strings.TrimPrefix(line, "- "))
		} else if len(section.Items) == 0 {
			section.Summary = strings.TrimSpace(section.Summary + " " + line)
		} else {
			return roadmap, fmt.Errorf("roadmap bullets must each occupy one line: %s", line)
		}
	}
	for i, title := range []string{"Done", "Next", "Future"} {
		if len(roadmap.Sections) != 3 || roadmap.Sections[i].Title != title || len(roadmap.Sections[i].Items) == 0 {
			return roadmap, fmt.Errorf("ROADMAP.md needs Done, Next, and Future sections with bullets")
		}
	}
	roadmap.DoneGroups = groupDone(roadmap.Sections[0].Items)
	return roadmap, nil
}

// groupDone derives the changelog grouping from data, not curation: a
// bullet's leading "Name:" prefix (colon plus space) names its group, groups
// keep first-appearance order, and prefix-less bullets or prefixes over 40
// bytes fall under "More" so a long sentence never becomes a heading.
func groupDone(items []string) []RoadmapGroup {
	index := map[string]int{}
	var groups []RoadmapGroup
	for _, item := range items {
		name := "More"
		if prefix, _, found := strings.Cut(item, ": "); found && prefix != "" && len(prefix) <= 40 {
			name = prefix
		}
		i, ok := index[name]
		if !ok {
			i = len(groups)
			index[name] = i
			groups = append(groups, RoadmapGroup{Name: name, Anchor: anchorSlug(name)})
		}
		groups[i].Items = append(groups[i].Items, item)
		groups[i].Count = len(groups[i].Items)
	}
	return groups
}

// anchorSlug turns a group name into the changelog page's stable anchor id.
func anchorSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteRune('-')
		}
	}
	return "done-" + strings.Trim(b.String(), "-")
}

// highlight marks up Zhex source with the design system's token classes.
// Every token's text and everything between tokens is HTML-escaped; only
// the spans themselves are added, so the result is safe trusted HTML.
// codeChips renders backticked spans as <code> chips. Every segment is
// HTML-escaped and only the code tags are added, so the result is safe
// trusted HTML.
func codeChips(text string) template.HTML {
	parts := strings.Split(text, "`")
	var out strings.Builder
	for i, part := range parts {
		if i%2 == 1 {
			out.WriteString("<code>")
		}
		out.WriteString(html.EscapeString(part))
		if i%2 == 1 {
			out.WriteString("</code>")
		}
	}
	return template.HTML(out.String())
}

func highlight(source string) template.HTML {
	runes := []rune(source)
	words := map[string]bool{}
	var out strings.Builder
	writeToken := func(class, text string) {
		out.WriteString(`<span class="` + class + `">` + html.EscapeString(text) + `</span>`)
	}
	identAt := func(i int) (string, int, bool) {
		if i >= len(runes) || !unicode.IsLetter(runes[i]) {
			return "", i, false
		}
		j := i + 1
		for j < len(runes) && (runes[j] == '-' || runes[j] == '.' || unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j])) {
			j++
		}
		return string(runes[i:j]), j, true
	}
	skipBlank := func(i int) int {
		for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t') {
			i++
		}
		return i
	}
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case r == '#':
			j := i
			for j < len(runes) && runes[j] != '\n' {
				j++
			}
			writeToken("tok-comment", string(runes[i:j]))
			i = j
		case r == '"':
			j := i + 1
			for j < len(runes) && runes[j] != '"' && runes[j] != '\n' {
				j++
				if runes[j-1] == '\\' {
					j++
				}
			}
			if j < len(runes) && runes[j] == '"' {
				j++
			}
			writeToken("tok-string", string(runes[i:j]))
			i = j
		case unicode.IsDigit(r):
			j := i + 1
			for j < len(runes) && unicode.IsDigit(runes[j]) {
				j++
			}
			writeToken("tok-number", string(runes[i:j]))
			i = j
		case unicode.IsLetter(r):
			text, j, _ := identAt(i)
			switch {
			case text == "flow":
				writeToken("tok-flow", text)
			case lang.Reserved(text):
				writeToken("tok-keyword", text)
				if text == "word" {
					if name, _, ok := identAt(skipBlank(j)); ok {
						words[name] = true
					}
				}
			case strings.ContainsRune(text, '.') || lang.HostWord(text):
				writeToken("tok-function", text)
			default:
				if k := skipBlank(j); k < len(runes) && runes[k] == '(' {
					writeToken("tok-function", text)
				} else if words[text] {
					writeToken("tok-function", text)
				} else if head, _ := utf8.DecodeRuneInString(text); unicode.IsUpper(head) {
					writeToken("tok-type", text)
				} else {
					writeToken("tok-variable", text)
				}
			}
			i = j
		default:
			out.WriteString(html.EscapeString(string(r)))
			i++
		}
	}
	return template.HTML(out.String())
}

// exampleSpecs is the human-facing manifest; its enforced twin is
// corpusTable in internal/wasm/corpus_test.go — the manifest is the matrix,
// that table is the gate; keep both in sync when an example's inputs change.
type exampleSpec struct {
	id, title, description, input, group string
	args                                 []string
	files                                []string // labeled library files beside the entry (a module)
	// kind is the run panel's input kind ("none", "int", or "text"),
	// lines the expected line count for int and text, and hint the short
	// human description of the entry; none rows leave it empty and the
	// generator renders the standard no-input sentence.
	kind  string
	lines int
	hint  string
}

var exampleSpecs = []exampleSpec{
	{"hello", "Console input", "Read a name, transform the value, and print a greeting.", "Ada\n", "Language basics", nil, nil, "text", 1, "One line of text"},
	{"composition", "Composition", "Build larger operations from small typed words.", "", "Language basics", nil, nil, "none", 0, ""},
	{"calls", "Calls and defaults", "Call words with or without parentheses and compose them in flows.", "", "Language basics", nil, nil, "none", 0, ""},
	{"word-defaults", "Word defaults", "Use inferred parameter defaults and fresh values for each call.", "", "Language basics", nil, nil, "none", 0, ""},
	{"partials", "Explicit partial application", "Capture arguments with holes, then call the resulting word value.", "", "Language basics", nil, nil, "none", 0, ""},
	{"records", "Named records", "Group related values in a checked named type.", "", "Language basics", nil, nil, "none", 0, ""},
	{"choices", "Choices and matching", "Select and handle alternatives with a checked match.", "", "Language basics", nil, nil, "none", 0, ""},
	{"collections", "Lists and arrays", "Build and safely iterate lists and rectangular arrays.", "", "Language basics", nil, nil, "none", 0, ""},
	{"temporal", "Dates and time zones", "Compare calendar days with elapsed time across a daylight-saving transition.", "", "Language basics", nil, nil, "none", 0, ""},
	{"json-config", "Typed JSON configuration", "Decode configuration with typed defaults and encode it again.", "", "Language basics", nil, nil, "none", 0, ""},
	{"json-tree", "JSON trees", "Parse a heterogeneous payload into a Json tree, walk it recursively, and encode it back.", "", "Language basics", nil, nil, "none", 0, ""},
	{"xml-config", "Typed XML configuration", "Decode an embedded XML document with attribute fields, a list of endpoint records, and an optional owner.", "", "Language basics", nil, nil, "none", 0, ""},
	{"maps", "Maps and lookup", "Store keyed values, match optional lookups, and merge maps.", "", "Language basics", nil, nil, "none", 0, ""},
	{"recovery", "Error recovery", "Catch invalid numeric input and continue with a useful fallback.", "", "Language basics", nil, nil, "none", 0, ""},
	{"exit-status", "Exit status", "Choose a valid status; invalid input exits with status 2.", "0\n", "Language basics", nil, nil, "int", 1, "One number"},
	{"text-stats", "Text statistics", "Read a file two ways: whole-buffer counts and a streamed uppercased echo.", "", "Language basics", []string{"examples/text-stats-sample.txt"}, nil, "none", 0, ""},
	{"line-filter", "Line filter", "Split embedded records, select them by literal prefix and suffix, and join a report.", "", "Language basics", nil, nil, "none", 0, ""},
	{"score-report", "Score report", "Trim and score embedded records with list map, filter, fold, find, any, and all.", "", "Language basics", nil, nil, "none", 0, ""},
	{"sorting", "Sorting", "Rank records with list.sort-by and a named key word, and sort plain text with list.sort.", "", "Language basics", nil, nil, "none", 0, ""},
	{"file-write", "File write", "Attempt a checked report write and handle the failed Result with catch.", "", "Language basics", nil, nil, "none", 0, ""},
	{"paths", "Lexical paths", "Compose and decompose paths with file.join, base, extension, and directory-of.", "", "Language basics", nil, nil, "none", 0, ""},
	{"file-work", "File work folder", "Create a work folder, write reports into it, inspect them, and remove the whole tree.", "", "Language basics", []string{"."}, nil, "none", 0, ""},
	{"env-config", "Env configuration", "Write a .env fixture, load it, read settings back as options, and clean up.", "", "Language basics", []string{"."}, nil, "none", 0, ""},
	{"modules/main", "Modules", "Split a program across files: main imports a sibling math library.", "48\n18\n", "Language basics", nil, []string{"math.zhex"}, "int", 2, "Two numbers, one per line"},
	{"modules/math.test", "Module tests", "Check a module's words from a .test.zhex file the zhex test runner discovers.", "", "Language basics", nil, []string{"math.zhex", "test.zhex"}, "none", 0, ""},
	{"modules/md-report/main", "Markdown report", "Compose records and lists into a Markdown report through a pure-Zhex md module.", "", "Language basics", nil, []string{"md.zhex"}, "none", 0, ""},
	{"modules/html-page/main", "HTML page", "Compose a page from the html module: text escapes by default, markup nests through element with one deliberate raw, and a javascript: link is rejected.", "", "Language basics", nil, []string{"html.zhex"}, "none", 0, ""},
	{"fizzbuzz", "FizzBuzz", "Combine conditions, integer remainders, and a bounded loop.", "15\n", "Algorithms", nil, nil, "int", 1, "One number"},
	{"fibonacci", "Fibonacci", "Use explicit mutable locals for an iterative algorithm.", "10\n", "Algorithms", nil, nil, "int", 1, "One number"},
	{"factorial", "Factorial", "Accumulate a result using a half-open integer range.", "5\n", "Algorithms", nil, nil, "int", 1, "One number"},
	{"times-table", "Times table", "Iterate a bounded range and build output strings from integers.", "7\n", "Algorithms", nil, nil, "int", 1, "One number"},
	{"palindrome", "Palindrome", "Compare text with its Unicode code-point reversal.", "racecar\n", "Algorithms", nil, nil, "text", 1, "One line of text"},
	{"gcd", "Greatest common divisor", "Recurse with Euclid's method and pipe a computed result through a flow.", "48\n18\n", "Algorithms", nil, nil, "int", 2, "Two numbers, one per line"},
	{"life", "Game of Life", "Carry a finite grid through each generation in a flow; cells beyond the board are dead.", "", "Algorithms", nil, nil, "none", 0, ""},
	{"edit-distance", "Edit distance", "Fill a dynamic-programming table to compare Unicode text.", "", "Algorithms", nil, nil, "none", 0, ""},
	{"collatz", "Collatz steps", "Count the steps each number takes to reach 1 with a while loop.", "", "Algorithms", nil, nil, "none", 0, ""},
}

// demoSpec is one demos-page program under examples/ (outside the examples
// page's spec list): the generator runs it for the oracle stdout and the
// -wasm pass compiles it to its own browser artifact. Title and blurb head
// the player on the page.
type demoSpec struct {
	id    string
	file  string // relative to examples/
	group string
	title string
	blurb string
}

// demoSpecs is the demos manifest; corpusTable in internal/wasm/corpus_test.go
// pins each file's corpus membership, and verify-wasm re-parses each demo's
// Output with the page's frame parser — keep all three in sync. The order is
// the page order (#116, trimmed by #118): the glider gun, the acorn, and
// diehard, grouped as Game of Life (#119) — the page heads each distinct
// group in first-appearance order, so more groups can arrive as new kinds of
// demo do.
var demoSpecs = []demoSpec{
	{"gun", "demos/gun.zhex", "Game of Life", "Gosper glider gun",
		"The classic gun fires a new glider every thirty generations; the stream keeps crossing the board, so the run never settles."},
	{"acorn", "demos/acorn.zhex", "Game of Life", "Acorn",
		"Seven cells explode into a drifting cloud of chaos — the methuselah's first 150 generations, the run's densest stretch."},
	{"diehard", "demos/diehard.zhex", "Game of Life", "Diehard",
		"Seven cells churn for exactly 130 generations and then vanish completely — the run ends on an empty board."},
}

// aocSpec is one Advent of Code day: the manifest row the loader turns
// into an AocDay. Statuses stay `empty` until a year's verification sweep
// flips them; day and title come from the epic's day tables, and the sample
// and solution paths follow the aoc/<year> convention, not the rows.
// Answers are the sample input's part 1 and part 2 results — the run panel
// compares against them; kind is the input-box vocabulary the examples use.
type aocSpec struct {
	year    int
	day     int
	title   string
	outline string
	status  string
	sample  string   // relative to aoc/2025/; empty until the day has one
	answers []string // sample answers, part 1 then part 2, once computed
	kind    string   // input box kind: text for every solved day so far
}

// aocSamplePath names the day's sample input file under aoc/<year>.
func aocSamplePath(day int) string { return fmt.Sprintf("samples/day%02d.txt", day) }

// aocSolutionPath names the day's solution under aoc/<year>.
func aocSolutionPath(day int) string { return fmt.Sprintf("day%02d.zhex", day) }

// aocSpecs is the AoC manifest — one block per archive year, oldest first:
// 2023's and 2024's 25 rows carry part-1 outlines, 2025 the twelve rows of
// the 12-day event; the page derives its tabs and the upcoming year's
// teaser from this order (#137, #138, #145).
var aocSpecs = append([]aocSpec{
	// Titles are the puzzle pages' own names; outlines are our words for
	// the input and part 1 only — part-two sentences join when a solving
	// lane solves the day (#138, #145).
	{2023, 1, "Trebuchet?!", "The input is a calibration document whose lines each hide digits among other characters. A line's value is the two-digit number formed from its first digit and its last digit. Part one asks for the sum of the values over all lines. Part two also counts digits spelled out as one through nine, where a word's last letter may start the next one, and asks for the sum again.", "solved", "samples/day01.txt", []string{"142", "142"}, "text"},
	{2023, 2, "Cube Conundrum", "The input lists games, each with an ID and a few sets of revealed cubes, every set naming how many red, green, and blue cubes came out of the bag. A game is possible when no set shows more than 12 red, 13 green, or 14 blue cubes at once. Part one asks for the sum of the IDs of the possible games. Part two asks, for each game, for the fewest cubes per color that could have been in the bag — the largest count the game revealed — and sums each game's red, green, and blue fewest multiplied together.", "solved", "samples/day02.txt", []string{"8", "2286"}, "text"},
	{2023, 3, "Gear Ratios", "The input is a grid of the engine schematic: numbers, periods, and single symbol characters. Any number adjacent to a symbol, even diagonally at a corner, counts as a part number. Part one asks for the sum of all part numbers. Part two calls an asterisk with exactly two adjacent part numbers a gear and asks for the sum of every gear's ratio, the two numbers multiplied together.", "solved", "samples/day03.txt", []string{"4361", "467835"}, "text"},
	{2023, 4, "Scratchcards", "Each input line is a scratchcard holding two lists of numbers separated by a bar: winning numbers, then the numbers on the card. A card scores one point for its first winning match, and each match after the first doubles its value. Part one asks for the total points across the pile. Part two stops scoring points and starts winning copies instead: each card wins one copy of each of the next cards, as many as it has matches, copies win the same way, and the answer is how many scratchcards end up in the pile, originals included.", "solved", "samples/day04.txt", []string{"13", "30"}, "text"},
	{2023, 5, "If You Give A Seed A Fertilizer", "The input lists seed numbers and a chain of maps — seed to soil, soil to fertilizer, and so on up to location — where each line converts a span of source numbers to a same-sized span of destination numbers, and unmapped numbers pass through unchanged. Part one asks for the lowest location number any seed reaches when carried through the whole chain. Part two rereads the seed line as pairs of a starting seed and a count, so the seeds describe huge ranges, and asks the same lowest location over every seed in every range.", "solved", "samples/day05.txt", []string{"35", "46"}, "text"},
	{2023, 6, "Wait For It", "The input pairs each race's time limit with its record distance. Holding the button down for some whole number of milliseconds sets the boat's speed to that many millimeters per millisecond for the rest of the race, so the distance covered is the hold time times the time left. Part one asks how many hold times beat the record in each race, and multiplies those counts together.", "empty", "", nil, ""},
	{2023, 7, "Camel Cards", "The input lists hands of five cards, each with a bid; hands order by type from five of a kind down to high card, with ties broken by comparing cards left to right. Each hand wins its bid times its rank in that ordering, weakest hand at rank 1. Part one asks for the total winnings across all hands.", "empty", "", nil, ""},
	{2023, 8, "Haunted Wasteland", "The input gives one line of left and right instructions, then a network of named nodes, each mapping to the two nodes its left and right sides lead to. Walking from node AAA by the instructions, starting them over whenever they run out, the goal is to reach node ZZZ. Part one asks how many steps the walk takes.", "empty", "", nil, ""},
	{2023, 9, "Mirage Maintenance", "Each input line is a history of numbers. Repeatedly writing the differences between neighboring values builds shorter sequences down to all zeroes, and the history's next value is worked back up from there. Part one asks for the sum of the next values over all histories.", "empty", "", nil, ""},
	{2023, 10, "Pipe Maze", "The input sketches a grid of pipe tiles, where each pipe shape joins two of the four directions and S marks the animal's starting tile; one continuous loop of pipe runs through the grid. Part one asks how many steps along the loop lie between S and the loop tile farthest from it.", "empty", "", nil, ""},
	{2023, 11, "Cosmic Expansion", "The input is a grid of empty space and galaxies, and every row or column holding no galaxy doubles in size before any distance is measured. Part one asks for the sum of the shortest path lengths between every pair of galaxies on the expanded grid.", "empty", "", nil, ""},
	{2023, 12, "Hot Springs", "Each input line shows a row of springs — . operational, # damaged, ? unknown — followed by the sizes of the damaged-spring groups in the order they appear. Part one asks for the total number of ways to fill in the unknown springs so every row matches its list of group sizes.", "empty", "", nil, ""},
	{2023, 13, "Point of Incidence", "The input holds grids of ash and rocks, one pattern per block, and each pattern has a reflection line between two rows or between two columns with one side matching the other exactly. Part one asks for a summary number: the columns left of each vertical reflection, plus 100 times the rows above each horizontal reflection, added over all patterns.", "empty", "", nil, ""},
	{2023, 14, "Parabolic Reflector Dish", "The input is a platform grid holding rounded rocks, cube-shaped rocks, and empty spaces. Tilting the platform north rolls every rounded rock upward until another rock or the edge stops it, while cube-shaped rocks never move. Part one asks for the total load on the north support beam after that tilt, where each rounded rock loads the beam by the count of rows from it to the platform's south edge, its own row included.", "empty", "", nil, ""},
	{2023, 15, "Lens Library", "The input is one comma-separated sequence of steps. The HASH algorithm turns a step's characters into a number from 0 to 255 by adding each character's code, multiplying by 17, and keeping the remainder of dividing by 256. Part one asks for the sum of the HASH results over all steps.", "empty", "", nil, ""},
	{2023, 16, "The Floor Will Be Lava", "The input is a grid of empty space, mirrors, and splitters, and one beam enters at the top left heading right, reflecting off mirrors and splitting in two at the flat side of a splitter. Part one asks how many tiles end up energized, meaning at least one beam passes, reflects, or splits there.", "empty", "", nil, ""},
	{2023, 17, "Clumsy Crucible", "The input is a grid of digits, each digit the heat loss of entering that city block, with the crucible starting at the top left and the factory at the bottom right. The crucible can go at most three blocks in one direction before it must turn left or right, and it never reverses. Part one asks for the least total heat loss of any route obeying those rules.", "empty", "", nil, ""},
	{2023, 18, "Lavaduct Lagoon", "The input is a dig plan of steps, each naming a direction and a distance in meters, and the steps trace one closed loop of trench. Part one asks how many cubic meters of lava the lagoon could hold once the loop's interior is dug out to one meter deep as well.", "empty", "", nil, ""},
	{2023, 19, "Aplenty", "The input has two sections: workflows whose rules test a part's four ratings and send it to another workflow, to accept, or to reject, and then the ratings of the parts to sort. Every part starts at the workflow named in. Part one asks for the sum of all four ratings added over the parts that end up accepted.", "empty", "", nil, ""},
	{2023, 20, "Pulse Propagation", "The input describes a network of modules: a broadcaster that forwards each pulse to all of its outputs, flip-flops that flip only on low pulses, and conjunctions that remember the latest pulse from each input and answer low only when every remembered pulse is high. Pressing the button sends one low pulse into the broadcaster, and all pulses finish processing before the next press. Part one asks for the product of the total low and high pulse counts after 1000 presses.", "empty", "", nil, ""},
	{2023, 21, "Step Counter", "The input maps garden plots, rocks, and a starting tile marked S, and each step moves onto an adjacent plot tile, never onto a rock. Part one asks how many plots the Elf can reach in exactly 64 steps.", "empty", "", nil, ""},
	{2023, 22, "Sand Slabs", "The input snapshots falling bricks of sand, each line giving the two end coordinates of one straight brick. Letting time run settles every brick down onto the ground or onto other bricks. Part one asks how many bricks could be removed one at a time with no other brick falling further as a result.", "empty", "", nil, ""},
	{2023, 23, "A Long Walk", "The input maps hiking trails: open path, forest, and slope arrows that force the next step to go the way the arrow points. A hike runs from the one open tile on the top row to the one on the bottom row without stepping on the same tile twice. Part one asks for the step count of the longest such hike.", "empty", "", nil, ""},
	{2023, 24, "Never Tell Me The Odds", "The input lists hailstones, each with a position in the air and a constant velocity. Watching only the X and Y axes, two hailstones' straight paths can cross in the future, have crossed in the past, or never cross. Part one asks how many pairs of paths cross in the future inside a large rectangular test area given in the puzzle text.", "empty", "", nil, ""},
	{2023, 25, "Snowverload", "The input is a wiring diagram listing, for each component, the components it connects to. Unplugging the right three wires splits the components into two separate groups. Part one asks for the product of the two groups' sizes.", "empty", "", nil, ""},
	// 2024's titles come from the puzzle pages too; same part-1-only rule.
	{2024, 1, "Historian Hysteria", "The input holds two lists of location IDs, one per column. Pair the smallest number in the left list with the smallest in the right, the second-smallest with the second-smallest, and so on, and measure how far apart each pair is. Part one asks for the total distance across all pairs.", "empty", "", nil, ""},
	{2024, 2, "Red-Nosed Reports", "The input lists reports, one per line, each a row of numbers called levels. A report is safe when its levels are all increasing or all decreasing and any two adjacent levels differ by at least one and at most three. Part one asks how many reports are safe.", "empty", "", nil, ""},
	{2024, 3, "Mull It Over", "The input is a stretch of corrupted computer memory holding letters, symbols, and a few intact mul(X, Y) instructions, where X and Y are one- to three-digit numbers. Part one asks for the sum of the products of all the intact instructions.", "empty", "", nil, ""},
	{2024, 4, "Ceres Search", "The input is a grid of letters, and the goal is a word search for XMAS. The word may run horizontally, vertically, or diagonally, forwards or backwards, and matches may overlap. Part one asks how many times XMAS appears in the grid.", "empty", "", nil, ""},
	{2024, 5, "Print Queue", "The input has two sections: page ordering rules, each naming two page numbers where the first must print before the second, and updates listing the pages to print in order. Rules about pages an update does not include are ignored. Part one asks for the sum of the middle page numbers of the updates already in the correct order.", "empty", "", nil, ""},
	{2024, 6, "Guard Gallivant", "The input maps a lab, with # marking obstructions and ^ marking the guard's start facing up. The guard steps forward, turning right 90 degrees whenever something is directly in front. Part one asks how many distinct positions the guard visits, start included, before leaving the map.", "empty", "", nil, ""},
	{2024, 7, "Bridge Repair", "Each input line is an equation: a test value before a colon, then a row of numbers. Inserting + or * between the numbers, evaluated left to right with no rearranging, has to produce the test value for the equation to be true. Part one asks for the sum of the test values of the equations that can be made true.", "empty", "", nil, ""},
	{2024, 8, "Resonant Collinearity", "The input maps antennas on a grid, where every letter and digit is one frequency, and uppercase and lowercase count separately. Each pair of same-frequency antennas creates two antinodes, one on either side, aligned with the pair and twice as far from one antenna as from the other. Part one asks how many unique in-bounds locations hold at least one antinode.", "empty", "", nil, ""},
	{2024, 9, "Disk Fragmenter", "The input is one line of digits alternating between file lengths and free-space lengths, with files numbered from 0 in order. Compacting moves one block at a time from the end of the disk into the leftmost free space until no gaps remain between file blocks. Part one asks for the checksum of the compacted disk: the sum of each file block's position times its file ID.", "empty", "", nil, ""},
	{2024, 10, "Hoof It", "The input is a topographic map of heights from 0 to 9. A hiking trail starts at a 0, ends at a 9, and goes up by exactly one at every step, moving only up, down, left, or right; a trailhead's score is the number of 9-height positions its trails can reach. Part one asks for the sum of the scores of all trailheads.", "empty", "", nil, ""},
	{2024, 11, "Plutonian Pebbles", "The input lists the numbers engraved on a line of stones. Every blink changes each stone at once: a 0 becomes 1, a stone with an even number of digits splits into two stones holding the left and right halves, and any other stone is replaced by its number multiplied by 2024. Part one asks how many stones the line holds after 25 blinks.", "empty", "", nil, ""},
	{2024, 12, "Garden Groups", "The input maps garden plots by plant letter, and plots of the same letter touching up, down, left, or right form a region. Part one asks for the total price of fencing every region: each region's area times its perimeter, added together.", "empty", "", nil, ""},
	{2024, 13, "Claw Contraption", "The input describes claw machines: each button A or B moves the claw a fixed distance along two axes, each machine holds one prize, and a prize is won only when the claw lines up with it exactly. A press of A costs 3 tokens and a press of B costs 1, with no button pressed more than 100 times per machine. Part one asks for the fewest total tokens that wins every winnable prize.", "empty", "", nil, ""},
	{2024, 14, "Restroom Redoubt", "The input lists robots in a 101-tile wide, 103-tile tall space, each with a starting position and a velocity in tiles per second, and robots wrap around at the edges without interacting. Part one asks for the safety factor after exactly 100 seconds: the four quadrant robot counts multiplied together, ignoring robots sitting on the middle row or column.", "empty", "", nil, ""},
	{2024, 15, "Warehouse Woes", "The input holds a warehouse map — # walls, O boxes, @ the robot — then the robot's sequence of moves. The robot pushes boxes ahead of it, and nothing moves when a box or the robot would hit a wall. Part one asks for the sum of the boxes' GPS coordinates after all moves, where a box's coordinate is 100 times its distance from the top edge plus its distance from the left edge.", "empty", "", nil, ""},
	{2024, 16, "Reindeer Maze", "The input maps a maze with a start marked S and an end marked E. A reindeer starts at S facing east, pays 1 point per step forward and 1000 points per 90-degree turn, and can never enter a wall. Part one asks for the lowest score any route from S to E can earn.", "empty", "", nil, ""},
	{2024, 17, "Chronospatial Computer", "The input gives the starting values of three registers and a program of 3-bit numbers run by a small computer with eight instructions, each reading a literal or combo operand. Part one asks you to run the program and join the values its out instruction produces with commas.", "empty", "", nil, ""},
	{2024, 18, "RAM Run", "The input lists grid coordinates, one per line, of bytes falling into a memory space whose coordinates run from 0 to 70 on both axes; each fallen byte corrupts its position, and corrupted positions cannot be entered. Part one asks for the fewest steps from the top-left corner to the bottom-right corner after the first 1024 bytes have fallen.", "empty", "", nil, ""},
	{2024, 19, "Linen Layout", "The input lists the available towel stripe patterns, then, after a blank line, the designs to display, each a sequence of stripe colors. A design is possible when towels with the available patterns can be laid in order so their stripes match the design exactly. Part one asks how many designs are possible.", "empty", "", nil, ""},
	{2024, 20, "Race Condition", "The input maps a racetrack of walls and track with a single start and end. Once per race a runner may switch off collision for up to two picoseconds, passing through walls as long as it lands back on track. Part one asks how many different cheats save at least 100 picoseconds.", "empty", "", nil, ""},
	{2024, 21, "Keypad Conundrum", "The input lists five codes to type on a door's numeric keypad. Your directional keypad moves a robot arm over a second directional keypad, whose presses move another arm over the numeric keypad, and no arm may ever aim at a gap between buttons, even for an instant. Part one asks for the sum of the codes' complexities, each the shortest number of presses you must type multiplied by the code's numeric part.", "empty", "", nil, ""},
	{2024, 22, "Monkey Market", "Each input line is one buyer's initial secret number. A buyer's next secret comes from three steps — multiply by 64, divide by 32 rounding down, multiply by 2048 — each mixed into the secret with a bitwise XOR and pruned to the remainder of dividing by 16777216. Part one asks for the sum of every buyer's 2000th new secret number.", "empty", "", nil, ""},
	{2024, 23, "LAN Party", "The input lists connections between named computers, and connections run both ways. Part one asks how many sets of three computers have every pair connected, counting only sets that include at least one computer whose name starts with t.", "empty", "", nil, ""},
	{2024, 24, "Crossed Wires", "The input holds starting wire values, then a list of AND, OR, and XOR gates, each reading two wires and writing one, with gates waiting for both inputs and no loops in the wiring. Part one asks for the decimal number formed by the bits on all wires whose names start with z, z00 being the least significant.", "empty", "", nil, ""},
	{2024, 25, "Code Chronicle", "The input holds schematics of locks, whose top row is filled, and keys, whose bottom row is filled, and each schematic reads as five column heights. A key fits a lock when, in every column, the lock's height and the key's height together stay within the available space. Part one asks how many lock and key pairs fit.", "empty", "", nil, ""},
},
	[]aocSpec{
		{2025, 1, "Secret Entrance", "The password for the North Pole base sits in a safe, and the input lists turns of the safe's dial: each turn is a direction, left or right, plus a number of clicks. The dial shows 0 to 99, starts at 50, and wraps around at both ends. Part one asks how many times the dial points at 0 right after a turn. Part two makes the turns enormous and asks how many times the dial passes over 0 anywhere inside a single turn, not only where it stops.", "solved", "samples/day01.txt", []string{"3", "6"}, "text"},
		{2025, 2, "Gift Shop", "The gift shop's price list is riddled with made-up product IDs, and the input is a list of ranges, each written as a first and last ID separated by a dash, all separated by commas. An ID is invalid when its digits are one block repeated exactly twice, like 55 or 123123, with no leading zero. Part one asks for the sum of every invalid ID found inside the ranges. Part two widens the rule: an ID now counts when its digits are one block repeated any number of times, so 555 and 123123123 join the list.", "solved", "samples/day02.txt", []string{"1227775554", "4174379265"}, "text"},
		{2025, 3, "Lobby", "The input gives one line of digits per bank of batteries, where each digit is a battery's power rating from 1 to 9. From a bank you switch on exactly two batteries, and their digits, kept in line order, form a two-digit number. Part one asks for the sum of the largest number each bank can produce. Part two switches on twelve batteries per bank instead of two, so each bank's output becomes the largest twelve-digit number its line order allows.", "solved", "samples/day03.txt", []string{"357", "3121910778619"}, "text"},
		{2025, 4, "Printing Department", "The input is a diagram of the printing department floor, with rolls of paper shown as @ marks on a grid. A roll counts as accessible when fewer than four of its eight neighboring positions hold rolls. Part one asks how many rolls on the grid are accessible. Part two keeps removing the accessible rolls round after round — each removal freeing its neighbors — and asks how many rolls come off the floor before the rest are stuck for good.", "solved", "samples/day04.txt", []string{"13", "43"}, "text"},
		{2025, 5, "Cafeteria", "The input has two sections: a list of fresh-ingredient ID ranges, then a blank line and a list of the ingredient IDs on hand, and the ranges overlap one another. An ID counts as fresh when it falls inside any of the ranges. Part one asks how many of the IDs on hand are fresh. Part two asks how many different IDs the ranges cover between them, with overlapping IDs counted only once.", "solved", "samples/day05.txt", []string{"3", "14"}, "text"},
		{2025, 6, "Trash Compactor", "Stuck under a trash compactor, you work through the cephalopods' math homework. The worksheet sets numbers in vertical columns, puts a + or * at the bottom of each problem, and separates problems with a column of spaces; spacing inside a problem can be ignored. Part one asks for the total of all the problems' answers. Part two reveals the numbers must be read the other way: each column of digits, top to bottom, is one number, and an all-space column separates the problems, so the total is taken again over that reading.", "solved", "samples/day06.txt", []string{"4277556", "3263827"}, "text"},
		{2025, 7, "Laboratories", "A broken teleporter reports a fault in its tachyon manifold, and the input draws that manifold: open space, splitter tiles marked ^, and one entry marked S. Beams enter at S and always travel down; a beam that reaches a splitter stops, and two new beams continue from the splitter's left and right sides. Part one asks for the total number of times the beams split. Part two asks how many beam timelines leave the bottom of the diagram, counting every route separately even after beams landing on the same spot merge.", "solved", "samples/day07.txt", []string{"21", "40"}, "text"},
		{2025, 8, "Playground", "The input lists the 3D coordinates of junction boxes hung in an underground playground, joined into circuits by strings. Each string joins the closest pair of boxes not yet directly connected; a string between boxes already in one circuit changes nothing. Part one asks for the product of the sizes of the three largest circuits once the 1000 shortest connections have been made. Part two keeps connecting the closest pairs until every box shares one circuit, then asks for the product of the X coordinates of the two boxes whose string completed it.", "solved", "samples/day08.txt", []string{"40", "25272"}, "text"},
		{2025, 9, "Movie Theater", "Elves are re-tiling a movie theater floor, and the input lists the coordinates of the red tiles. Any two red tiles can serve as opposite corners of a rectangle of new tiles. Part one asks for the largest area such a rectangle can cover. Part two connects the red tiles in input order into a closed loop of edges and asks for the largest rectangle whose tiles all stay inside that loop.", "solved", "samples/day09.txt", []string{"50", "24"}, "text"},
		{2025, 10, "Factory", "Each input line describes one broken machine: a light diagram in brackets, button schematics in parentheses naming which lights each press toggles, and joltage requirements in braces. Every machine's lights start off, and each press flips its listed lights on or off. Part one ignores the joltage requirements and asks for the fewest total button presses that set every machine's lights to its target pattern. Part two makes each light's joltage number the exact number of times that light must change state, and asks again for the fewest total button presses.", "solved", "samples/day10.txt", []string{"7", "33"}, "text"},
		{2025, 11, "Reactor", "The input maps the wiring of a reactor under a factory floor: each line names a device and lists the devices its output feeds, and data never flows backward. Part one asks for the number of distinct routes from the device named you to the device named out. Part two asks for the number of routes from the device named svr to out that pass through both the device named dac and the device named fft.", "solved", "samples/day11.txt", []string{"5", "0"}, "text"},
		{2025, 12, "Christmas Tree Farm", "The input lists six present shapes as small grids, then a set of regions, each sized width by length and followed by how many of each shape should fit inside. Presents can be rotated or flipped, must sit on the grid, may not overlap, and cannot stack. Part one asks how many regions can hold all the presents listed for them. Part two is the twelve-day event's finale: with every star collected it asks nothing further, and the customary completion answer is the star itself.", "solved", "samples/day12.txt", []string{"2", "*"}, "text"},
	}...)

func main() {
	wasmDir := flag.String("wasm", "", "compile each example's browser WebAssembly artifact into this directory")
	flag.Parse()
	data, err := loadExamples(".")
	if err == nil {
		data.Demos, err = loadDemos(".")
	}
	if err == nil {
		data.Aoc, err = loadAoc(".")
	}
	if err == nil {
		data.Roadmap, err = readRoadmap("ROADMAP.md")
	}
	if err == nil {
		var source []byte
		source, err = os.ReadFile("docs/manifesto.md")
		if err == nil {
			data.Manifesto, err = loadManifesto(source)
		}
	}
	if err == nil {
		data.Catalog, err = loadCatalog()
	}
	if err == nil && *wasmDir != "" {
		err = compileExamples(&data, *wasmDir)
	}
	if err == nil {
		err = json.NewEncoder(os.Stdout).Encode(data)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readRoadmap(path string) (Roadmap, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return Roadmap{}, err
	}
	return loadRoadmap(source)
}

// validateSpec enforces the run-panel declaration: the kind must classify
// the row and agree with the stdin it pairs with; the page validates the
// box against it, so a wrong declaration would teach visitors the wrong
// entry.
func validateSpec(spec exampleSpec) error {
	if spec.kind != "none" && spec.kind != "int" && spec.kind != "text" {
		return fmt.Errorf("example %s: kind must be none, int, or text", spec.id)
	}
	if spec.kind != "none" && spec.hint == "" {
		return fmt.Errorf("example %s: kind %s needs a hint", spec.id, spec.kind)
	}
	if spec.kind != "none" && spec.lines < 1 {
		return fmt.Errorf("example %s: kind %s needs at least one line", spec.id, spec.kind)
	}
	if stdinLines := strings.Count(spec.input, "\n"); (spec.kind == "none" && stdinLines != 0) || (spec.kind != "none" && stdinLines != spec.lines) {
		return fmt.Errorf("example %s: stdin %q disagrees with kind %s of %d lines", spec.id, spec.input, spec.kind, spec.lines)
	}
	return nil
}

func loadExamples(root string) (pageData, error) {
	var data pageData
	for _, spec := range exampleSpecs {
		if err := validateSpec(spec); err != nil {
			return pageData{}, err
		}
		kind, hint := spec.kind, spec.hint
		if kind == "none" {
			hint = "No input — this example is self-contained"
		}
		filename := spec.id + ".zhex"
		path := filepath.Join(root, "examples", filename)
		source, err := os.ReadFile(path)
		if err != nil {
			return pageData{}, fmt.Errorf("read example %s: %w", filename, err)
		}
		// Modules resolve as sibling files next to the entry, the rule the
		// interpreter applies; single-file examples never call the resolver.
		resolve := resolveSibling(path)
		var output bytes.Buffer
		runArgs := make([]string, len(spec.args))
		for i, arg := range spec.args {
			runArgs[i] = filepath.Join(root, arg)
		}
		if err := lang.RunWithModules(filename, string(source), strings.NewReader(spec.input), &output, nil, runArgs, resolve); err != nil {
			var exit *lang.ExitError
			if !errors.As(err, &exit) || exit.Status != 0 {
				return pageData{}, fmt.Errorf("run website example %s: %w", filename, err)
			}
		}
		command := "zhex run examples/" + filename
		for _, arg := range spec.args {
			command += " " + arg
		}
		example := Example{
			ID: spec.id, Title: spec.title, Description: spec.description,
			Group:  spec.group,
			Source: string(source), Highlighted: string(highlight(string(source))), Input: strings.TrimSuffix(spec.input, "\n"), Stdin: spec.input, Output: output.String(),
			InputKind: kind, InputLines: spec.lines, InputHint: hint,
			Command:   command,
			SourceURL: "examples/" + filename,
			File:      filepath.Base(filename),
		}
		for _, file := range spec.files {
			moduleSource, err := os.ReadFile(filepath.Join(filepath.Dir(path), file))
			if err != nil {
				return pageData{}, fmt.Errorf("read example %s: %w", file, err)
			}
			url := filepath.ToSlash(filepath.Join("examples", filepath.Dir(filename), file))
			example.Modules = append(example.Modules, ExampleModule{File: file, Source: string(moduleSource), URL: url, Highlighted: string(highlight(string(moduleSource)))})
		}
		data.Examples = append(data.Examples, example)
		if spec.id == "hello" {
			data.HeroSource = string(source)
			data.HeroInput = strings.TrimSuffix(spec.input, "\n")
			data.HeroOutput = output.String()
		}
	}
	return data, nil
}

// loadDemos runs every demo for its oracle stdout; the -wasm pass compiles
// the same programs to browser artifacts beside the examples'.
func loadDemos(root string) ([]Demo, error) {
	var demos []Demo
	for _, spec := range demoSpecs {
		if spec.group == "" {
			return nil, fmt.Errorf("demo %s: missing group", spec.id)
		}
		path := filepath.Join(root, "examples", spec.file)
		source, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read demo %s: %w", spec.file, err)
		}
		var output bytes.Buffer
		if err := lang.RunWithModules(path, string(source), strings.NewReader(""), &output, nil, nil, resolveSibling(path)); err != nil {
			return nil, fmt.Errorf("run demo %s: %w", spec.file, err)
		}
		demos = append(demos, Demo{ID: spec.id, Group: spec.group, Title: spec.title, Blurb: spec.blurb, Stdin: "", Output: output.String()})
	}
	return demos, nil
}

// loadAoc turns the manifest into AocDay rows: statuses are validated now
// so a later flip cannot drift silently — a non-empty status must find the
// day's sample under aoc/2025/samples, and a part1 or solved day must also
// find its solution, which runs once here for the Go oracle stdout the
// page's build gate compares the browser run against (#128).
func loadAoc(root string) ([]AocDay, error) {
	var days []AocDay
	for _, spec := range aocSpecs {
		switch spec.status {
		case "empty", "working", "part1", "solved":
		default:
			return nil, fmt.Errorf("aoc %d day %d: status must be empty, working, part1, or solved", spec.year, spec.day)
		}
		day := AocDay{
			Year:            spec.year,
			Day:             spec.day,
			Title:           spec.title,
			Outline:         spec.outline,
			OfficialURL:     fmt.Sprintf("https://adventofcode.com/%d/day/%d", spec.year, spec.day),
			Status:          spec.status,
			Sample:          spec.sample,
			ExpectedAnswers: spec.answers,
			InputKind:       spec.kind,
		}
		if spec.status != "empty" {
			if day.Sample == "" {
				day.Sample = aocSamplePath(spec.day)
			}
			sample, err := os.ReadFile(filepath.Join(root, "aoc", fmt.Sprintf("%d", spec.year), day.Sample))
			if err != nil {
				return nil, fmt.Errorf("read aoc %d day %d sample: %w", spec.year, spec.day, err)
			}
			day.Stdin = string(sample)
		}
		if spec.status == "part1" || spec.status == "solved" {
			runPath := filepath.Join("aoc", fmt.Sprintf("%d", spec.year), aocSolutionPath(spec.day))
			source, err := os.ReadFile(filepath.Join(root, runPath))
			if err != nil {
				return nil, fmt.Errorf("read aoc %d day %d solution: %w", spec.year, spec.day, err)
			}
			var output bytes.Buffer
			if err := lang.RunWithModules(runPath, string(source), strings.NewReader(day.Stdin), &output, nil, nil, resolveSibling(runPath)); err != nil {
				return nil, fmt.Errorf("run aoc %d day %d solution: %w", spec.year, spec.day, err)
			}
			day.Output = output.String()
		}
		days = append(days, day)
	}
	return days, nil
}

// resolveSibling builds the module resolver the interpreter applies: `use
// name` reads the sibling name.zhex next to the entry file.
func resolveSibling(entryPath string) func(string) (lang.ModuleSource, bool) {
	dir := filepath.Dir(entryPath)
	return func(name string) (lang.ModuleSource, bool) {
		path := filepath.Join(dir, name+".zhex")
		module, err := os.ReadFile(path)
		if err != nil {
			return lang.ModuleSource{}, false
		}
		return lang.ModuleSource{Path: path, Source: string(module)}, true
	}
}

// compileExamples writes each in-subset example's browser WebAssembly
// artifact into dir (flat names: modules/main becomes modules-main.wasm,
// demos join as demo-<id>.wasm) and records on the matching Example the
// relative artifact URL (Wasm) or the fence diagnostic message (WasmNote).
// Any other failure — a missing assembler, an emitter bug — fails the build
// loudly: no silent button-less site.
func compileExamples(data *pageData, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, spec := range exampleSpecs {
		example := &data.Examples[i]
		path := filepath.Join("examples", spec.id+".zhex")
		name := strings.ReplaceAll(spec.id, "/", "-") + ".wasm"
		err := wasm.Compile(path, example.Source, filepath.Join(dir, name), resolveSibling(path), lang.WatSkinBrowser)
		if err == nil {
			example.Wasm = "wasm/" + name
			continue
		}
		reportJSON, ok := lang.Report(err)
		var report struct{ Code, Message string }
		if !ok || json.Unmarshal([]byte(reportJSON), &report) != nil || report.Code != string(lang.CodeNotAvailableInBuild) {
			return fmt.Errorf("compile example %s: %w", spec.id+".zhex", err)
		}
		example.WasmNote = report.Message
	}
	// A demo artifact is required, never fenced: a demos page without its
	// artifact fails the build loudly, like a missing example compile does.
	for i, spec := range demoSpecs {
		path := filepath.Join("examples", spec.file)
		source, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read demo %s: %w", spec.file, err)
		}
		name := "demo-" + spec.id + ".wasm"
		if err := wasm.Compile(path, string(source), filepath.Join(dir, name), resolveSibling(path), lang.WatSkinBrowser); err != nil {
			return fmt.Errorf("compile demo %s: %w", spec.file, err)
		}
		data.Demos[i].Wasm = "wasm/" + name
	}
	// A solved door's run panel needs its artifact; a solution outside the
	// compiled subset takes the fence note naming the construct instead —
	// exactly like a fenced example, never a fake run. Empty and working
	// days compile nothing: their doors keep the honest status line (#128).
	for i, day := range data.Aoc {
		if day.Status != "part1" && day.Status != "solved" {
			continue
		}
		path := filepath.Join("aoc", fmt.Sprintf("%d", day.Year), aocSolutionPath(day.Day))
		source, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read aoc %d day %d solution: %w", day.Year, day.Day, err)
		}
		name := fmt.Sprintf("aoc-%d-day%02d.wasm", day.Year, day.Day)
		err = wasm.Compile(path, string(source), filepath.Join(dir, name), resolveSibling(path), lang.WatSkinBrowser)
		if err == nil {
			data.Aoc[i].Wasm = "wasm/" + name
			continue
		}
		reportJSON, ok := lang.Report(err)
		var report struct{ Code, Message string }
		if !ok || json.Unmarshal([]byte(reportJSON), &report) != nil || report.Code != string(lang.CodeNotAvailableInBuild) {
			return fmt.Errorf("compile aoc %d day %d: %w", day.Year, day.Day, err)
		}
		data.Aoc[i].WasmNote = report.Message
	}
	return nil
}
