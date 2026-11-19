package zhexrt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"unicode/utf8"
)

// stdin is the shared buffered reader, like the interpreter runtime's
// single input: read-line and file reads never interleave mid-line.
var stdin = bufio.NewReader(os.Stdin)

// chomp strips one trailing newline pair, the interpreter's read-line.
// Chomp drops the newline and a trailing carriage return.
func Chomp(line string) string {
	if n := len(line); n > 0 && line[n-1] == '\n' {
		line = line[:n-1]
		if n = len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
	}
	return line
}

func write(pos Pos, text string) {
	if _, err := io.WriteString(os.Stdout, text); err != nil {
		FailOp(pos, "console-output", "print", "console output failed: %v", err)
	}
}

// Print writes one line of text; PrintInt renders an Int the way
// int.to-string does; PrintBool renders true/false. All three return
// Unit so generated call sites can use the call's value.
func Print(pos Pos, text string) Unit {
	write(pos, text+"\n")
	return Unit{}
}

func PrintInt(pos Pos, n Int) Unit {
	write(pos, Str(n)+"\n")
	return Unit{}
}

func PrintBool(pos Pos, value bool) Unit {
	write(pos, strconv.FormatBool(value)+"\n")
	return Unit{}
}

// ReadLine writes the prompt (if any) and reads one chomped line. End of
// input fails with reason end-of-file, a genuine read failure with
// read-failed (read with Error.Detail).
func ReadLine(pos Pos, prompt string) Result[string] {
	if prompt != "" {
		write(pos, prompt)
	}
	line, err := stdin.ReadString('\n')
	if err == io.EOF && len(line) == 0 {
		return ErrAt[string](pos, "console-input", "end of input", map[string]string{"reason": "end-of-file"})
	}
	if err != nil && err != io.EOF {
		return ErrAt[string](pos, "console-input", fmt.Sprintf("console input failed: %v", err), map[string]string{"reason": "read-failed"})
	}
	line = Chomp(line)
	if !utf8.ValidString(line) {
		return ErrAt[string](pos, "console-utf8", "console input is not valid UTF-8", nil)
	}
	return Ok(line)
}

// ToInt is text.to-int.
func ToInt(pos Pos, text string) Result[Int] {
	number, ok := Parse(text)
	if !ok {
		return ErrAt[Int](pos, "text-to-int", fmt.Sprintf("%q is not a signed 128-bit decimal integer", text), nil)
	}
	return Ok(number)
}
