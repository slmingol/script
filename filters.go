package script

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Column reads from the pipe, and returns a new pipe containing only the Nth
// column of each line in the input, where '1' means the first column, and
// columns are delimited by whitespace. Specifically, whatever Unicode defines
// as whitespace ('WSpace=yes'). If there is an error reading the pipe, the
// pipe's error status is also set.
func (p *Pipe) Column(col int) *Pipe {
	return p.EachLine(func(line string, out *strings.Builder) {
		columns := strings.Fields(line)
		if col <= len(columns) {
			out.WriteString(columns[col-1])
			out.WriteRune('\n')
		}
	})
}

// Concat reads a list of filenames from the pipe, one per line, and returns a
// pipe which reads all those files in sequence. If there are any errors (for
// example, non-existent files), the pipe's error status will be set to the
// first error encountered, but execution will continue.
func (p *Pipe) Concat() *Pipe {
	if p == nil || p.Error() != nil {
		return p
	}
	var readers []io.Reader
	p.EachLine(func(line string, out *strings.Builder) {
		input, err := os.Open(line)
		if err != nil {
			p.SetError(err)
			return
		}
		readers = append(readers, NewReadAutoCloser(input))
	})
	return p.WithReader(io.MultiReader(readers...))
}

// EachLine calls the specified function for each line of input, passing it the
// line as a string, and a *strings.Builder to write its output to. The return
// value from EachLine is a pipe containing the contents of the strings.Builder.
func (p *Pipe) EachLine(process func(string, *strings.Builder)) *Pipe {
	if p == nil || p.Error() != nil {
		return p
	}
	scanner := bufio.NewScanner(p.Reader)
	output := strings.Builder{}
	for scanner.Scan() {
		process(scanner.Text(), &output)
	}
	err := scanner.Err()
	if err != nil {
		p.SetError(err)
	}
	return Echo(output.String())
}

// Exec runs an external command and returns a pipe containing the output. If
// the command had a non-zero exit status, the pipe's error status will also be
// set to the string "exit status X", where X is the integer exit status.
func (p *Pipe) Exec(s string) *Pipe {
	if p == nil || p.Error() != nil {
		return p
	}
	q := NewPipe()
	args := strings.Fields(s)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = p.Reader
	output, err := cmd.CombinedOutput()
	if err != nil {
		q.SetError(err)
	}
	return q.WithReader(bytes.NewReader(output))
}

// First reads from the pipe, and returns a new pipe containing only the first N
// lines. If there is an error reading the pipe, the pipe's error status is also
// set.
func (p *Pipe) First(lines int) *Pipe {
	if p == nil || p.Error() != nil {
		return p
	}
	scanner := bufio.NewScanner(p.Reader)
	output := strings.Builder{}
	for i := 0; i < lines; i++ {
		if !scanner.Scan() {
			break
		}
		output.WriteString(scanner.Text())
		output.WriteRune('\n')
	}
	p.Close()
	err := scanner.Err()
	if err != nil {
		p.SetError(err)
	}
	return Echo(output.String())
}

// Freq reads from the pipe, and returns a new pipe containing only unique lines
// from the input, prefixed with a frequency count, in descending numerical
// order (most frequent lines first). Lines with equal frequency will be sorted
// alphabetically. If there is an error reading the pipe, the pipe's error
// status is also set.
func (p *Pipe) Freq() *Pipe {
	if p == nil || p.Error() != nil {
		return p
	}
	var freq = map[string]int{}
	p.EachLine(func(line string, out *strings.Builder) {
		freq[line]++
	})
	type frequency struct {
		line  string
		count int
	}
	var freqs = make([]frequency, 0, len(freq))
	var maxCount int
	for line, count := range freq {
		freqs = append(freqs, frequency{line, count})
		if count > maxCount {
			maxCount = count
		}
	}
	sort.Slice(freqs, func(i, j int) bool {
		if freqs[i].count == freqs[j].count {
			return freqs[i].line < freqs[j].line
		}
		return freqs[i].count > freqs[j].count
	})
	fieldWidth := len(strconv.Itoa(maxCount))
	var output strings.Builder
	for _, item := range freqs {
		output.WriteString(fmt.Sprintf("%*d %s", fieldWidth, item.count, item.line))
		output.WriteRune('\n')
	}
	return Echo(output.String())
}

// Join reads the contents of the pipe, line by line, and joins them into a
// single space-separated string. It returns a pipe containing this string. Any
// terminating newline is preserved.
func (p *Pipe) Join() *Pipe {
	if p == nil || p.Error() != nil {
		return p
	}
	result, err := p.String()
	if err != nil {
		return p
	}
	var terminator string
	if strings.HasSuffix(result, "\n") {
		terminator = "\n"
		result = result[:len(result)-1]
	}
	output := strings.ReplaceAll(result, "\n", " ")
	return Echo(output + terminator)
}

// Match reads from the pipe, and returns a new pipe containing only lines which
// contain the specified string. If there is an error reading the pipe, the
// pipe's error status is also set.
func (p *Pipe) Match(s string) *Pipe {
	return p.EachLine(func(line string, out *strings.Builder) {
		if strings.Contains(line, s) {
			out.WriteString(line)
			out.WriteRune('\n')
		}
	})
}

// MatchRegexp reads from the pipe, and returns a new pipe containing only lines
// which match the specified compiled regular expression. If there is an error
// reading the pipe, the pipe's error status is also set.
func (p *Pipe) MatchRegexp(re *regexp.Regexp) *Pipe {
	return p.EachLine(func(line string, out *strings.Builder) {
		if re.MatchString(line) {
			out.WriteString(line)
			out.WriteRune('\n')
		}
	})
}

// Reject reads from the pipe, and returns a new pipe containing only lines
// which do not contain the specified string. If there is an error reading the
// pipe, the pipe's error status is also set.
func (p *Pipe) Reject(s string) *Pipe {
	return p.EachLine(func(line string, out *strings.Builder) {
		if !strings.Contains(line, s) {
			out.WriteString(line)
			out.WriteRune('\n')
		}
	})
}

// RejectRegexp reads from the pipe, and returns a new pipe containing only
// lines which don't match the specified compiled regular expression. If there
// is an error reading the pipe, the pipe's error status is also set.
func (p *Pipe) RejectRegexp(re *regexp.Regexp) *Pipe {
	return p.EachLine(func(line string, out *strings.Builder) {
		if !re.MatchString(line) {
			out.WriteString(line)
			out.WriteRune('\n')
		}
	})
}
