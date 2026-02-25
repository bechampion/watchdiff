package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"
)

const (
	red      = "\033[1;91m"
	green    = "\033[1;92m"
	cyan     = "\033[1;96m"
	dim      = "\033[2;37m"
	bold     = "\033[1;97m"
	yellow   = "\033[1;93m"
	redBg    = "\033[1;97;41m"
	greenBg  = "\033[1;97;42m"
	dimRed   = "\033[91m"
	dimGreen = "\033[92m"
	gray     = "\033[90m"
	reset    = "\033[0m"
)

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func run(command string) string {
	cmd := exec.Command("sh", "-c", command)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func toLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func similarity(a, b string) float64 {
	wa := strings.Fields(a)
	wb := strings.Fields(b)
	if len(wa) == 0 && len(wb) == 0 {
		return 1.0
	}
	if len(wa) == 0 || len(wb) == 0 {
		return 0.0
	}

	set := make(map[string]bool)
	for _, w := range wa {
		set[w] = true
	}
	shared := 0
	for _, w := range wb {
		if set[w] {
			shared++
		}
	}
	total := len(wa)
	if len(wb) > total {
		total = len(wb)
	}
	return float64(shared) / float64(total)
}

func wordDiff(old, new string) (string, string) {
	oldWords := strings.Fields(old)
	newWords := strings.Fields(new)

	oldSet := make(map[string]int)
	for _, w := range oldWords {
		oldSet[w]++
	}
	newSet := make(map[string]int)
	for _, w := range newWords {
		newSet[w]++
	}

	var oldOut, newOut strings.Builder
	for i, w := range oldWords {
		if i > 0 {
			oldOut.WriteString(" ")
		}
		if newSet[w] > 0 {
			newSet[w]--
			oldOut.WriteString(dimRed + w + reset)
		} else {
			oldOut.WriteString(redBg + w + reset)
		}
	}

	newSet2 := make(map[string]int)
	for _, w := range oldWords {
		newSet2[w]++
	}

	for i, w := range newWords {
		if i > 0 {
			newOut.WriteString(" ")
		}
		if newSet2[w] > 0 {
			newSet2[w]--
			newOut.WriteString(dimGreen + w + reset)
		} else {
			newOut.WriteString(greenBg + w + reset)
		}
	}

	return oldOut.String(), newOut.String()
}

type diffResult struct {
	kind    string
	line    string
	oldLine string
	newLine string
}

func computeDiff(prev, curr []string) []diffResult {
	// Count prev line occurrences for matching unchanged lines
	prevCount := make(map[string]int)
	for _, line := range prev {
		prevCount[line]++
	}

	// Walk curr to classify each line as unchanged or added
	type lineInfo struct {
		line   string
		status string // "unchanged" or "added"
	}
	currInfo := make([]lineInfo, len(curr))

	for i, line := range curr {
		if prevCount[line] > 0 {
			prevCount[line]--
			currInfo[i] = lineInfo{line: line, status: "unchanged"}
		} else {
			currInfo[i] = lineInfo{line: line, status: "added"}
		}
	}

	// Remaining prev lines (prevCount > 0) are removed
	// Rebuild removed list preserving prev order
	prevCount2 := make(map[string]int)
	for _, line := range prev {
		prevCount2[line]++
	}
	for _, info := range currInfo {
		if info.status == "unchanged" {
			prevCount2[info.line]--
		}
	}
	var removedLines []string
	for _, line := range prev {
		if prevCount2[line] > 0 {
			removedLines = append(removedLines, line)
			prevCount2[line]--
		}
	}

	// Collect curr indices of added lines
	var addedIndices []int
	for i, info := range currInfo {
		if info.status == "added" {
			addedIndices = append(addedIndices, i)
		}
	}

	// Pair removed lines with similar added lines
	pairedAdded := make(map[int]int)   // curr index -> removedLines index
	pairedRemoved := make(map[int]bool) // removedLines index -> used

	for ri, rem := range removedLines {
		bestCurrIdx := -1
		bestSim := 0.4
		for _, ci := range addedIndices {
			if _, ok := pairedAdded[ci]; ok {
				continue
			}
			sim := similarity(rem, curr[ci])
			if sim > bestSim {
				bestSim = sim
				bestCurrIdx = ci
			}
		}
		if bestCurrIdx >= 0 {
			pairedAdded[bestCurrIdx] = ri
			pairedRemoved[ri] = true
		}
	}

	// Build results: unpaired removed lines first, then curr in order
	var results []diffResult

	for ri, rem := range removedLines {
		if !pairedRemoved[ri] {
			results = append(results, diffResult{kind: "removed", line: rem})
		}
	}

	for i, info := range currInfo {
		if info.status == "unchanged" {
			results = append(results, diffResult{kind: "unchanged", line: info.line})
		} else {
			// Added line — check if paired with a removed line
			if ri, ok := pairedAdded[i]; ok {
				oldHL, newHL := wordDiff(removedLines[ri], info.line)
				results = append(results, diffResult{kind: "changed", oldLine: oldHL, newLine: newHL})
			} else {
				results = append(results, diffResult{kind: "added", line: info.line})
			}
		}
	}

	return results
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: watchdiff [-n seconds] command\n")
	fmt.Fprintf(os.Stderr, "\nRuns a command repeatedly and shows colored diff output.\n")
	fmt.Fprintf(os.Stderr, "  %s- removed%s\n", red, reset)
	fmt.Fprintf(os.Stderr, "  %s+ added%s\n", green, reset)
	fmt.Fprintf(os.Stderr, "  Changed words are %shighlighted%s\n", redBg, reset)
	fmt.Fprintf(os.Stderr, "  %s~ unchanged%s\n", gray, reset)
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	fmt.Fprintf(os.Stderr, "  -n seconds   Interval between runs (default: 1, supports decimals like 0.5)\n")
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  watchdiff 'lsof -i tcp:5432 -n -P'\n")
	fmt.Fprintf(os.Stderr, "  watchdiff -n 0.5 'ps aux | grep postgres'\n")
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	interval := 1.0
	command := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n":
			if i+1 >= len(args) {
				usage()
			}
			i++
			if _, err := fmt.Sscanf(args[i], "%f", &interval); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid interval: %s\n", args[i])
				os.Exit(1)
			}
		case "-h", "--help":
			usage()
		default:
			command = strings.Join(args[i:], " ")
			i = len(args)
		}
	}

	if command == "" {
		usage()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	fmt.Printf("👀 %swatchdiff%s %s%s%s every %s%.1fs%s\n\n",
		bold, reset, cyan, command, reset, yellow, interval, reset)

	// Run once and print initial output
	out := run(command)
	prev := toLines(out)
	if strings.TrimSpace(out) == "" {
		fmt.Printf("%s--no output--%s\n\n", dim, reset)
	} else {
		fmt.Printf("%s%s%s", yellow, out, reset)
		if !strings.HasSuffix(out, "\n") {
			fmt.Println()
		}
		fmt.Println()
	}

	tick := time.Duration(interval * float64(time.Second))
	frame := 0

	spinTick := time.NewTicker(80 * time.Millisecond)
	defer spinTick.Stop()

	// Run command in background so it never blocks the spinner
	type cmdResult struct {
		lines []string
	}
	resultCh := make(chan cmdResult, 1)

	runAsync := func() {
		go func() {
			out := run(command)
			resultCh <- cmdResult{lines: toLines(out)}
		}()
	}

	// Schedule first run after interval
	time.AfterFunc(tick, runAsync)

	fmt.Printf("\r%s%s %swatching...%s", cyan, spinner[0], dim, reset)

	for {
		select {
		case <-sig:
			fmt.Printf("\r\033[K%s", reset)
			os.Exit(0)

		case <-spinTick.C:
			frame++
			s := spinner[frame%len(spinner)]
			fmt.Printf("\r%s%s %swatching...%s", cyan, s, dim, reset)

		case res := <-resultCh:
			curr := res.lines

			if prev != nil {
				results := computeDiff(prev, curr)

				// Only print if there are actual changes (not just unchanged lines)
				hasChanges := false
				for _, r := range results {
					if r.kind != "unchanged" {
						hasChanges = true
						break
					}
				}

				if hasChanges {
					fmt.Printf("\r\033[K")

					now := time.Now().Format("15:04:05")
					fmt.Printf("%s%s [%s]%s\n", dim, now, command, reset)

					for _, r := range results {
						switch r.kind {
						case "removed":
							fmt.Printf("  %s- %s%s\n", red, r.line, reset)
						case "added":
							fmt.Printf("  %s+ %s%s\n", green, r.line, reset)
						case "changed":
							fmt.Printf("  - %s\n", r.oldLine)
							fmt.Printf("  + %s\n", r.newLine)
						case "unchanged":
							fmt.Printf("  %s~ %s%s\n", gray, r.line, reset)
						}
					}

					fmt.Println()
				}
			}

			prev = curr

			// Schedule next run
			time.AfterFunc(tick, runAsync)
		}
	}
}
