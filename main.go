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
	var removed []string
	tempPrev := make(map[string]int)
	for _, line := range prev {
		tempPrev[line]++
	}
	for _, line := range curr {
		if tempPrev[line] > 0 {
			tempPrev[line]--
		}
	}
	for _, line := range prev {
		if tempPrev[line] > 0 {
			removed = append(removed, line)
			tempPrev[line]--
		}
	}

	var added []string
	tempCurr := make(map[string]int)
	for _, line := range prev {
		tempCurr[line]++
	}
	for _, line := range curr {
		if tempCurr[line] > 0 {
			tempCurr[line]--
		} else {
			added = append(added, line)
		}
	}

	var results []diffResult
	usedAdded := make(map[int]bool)

	for _, rem := range removed {
		bestIdx := -1
		bestSim := 0.4
		for j, add := range added {
			if usedAdded[j] {
				continue
			}
			sim := similarity(rem, add)
			if sim > bestSim {
				bestSim = sim
				bestIdx = j
			}
		}

		if bestIdx >= 0 {
			oldHighlighted, newHighlighted := wordDiff(rem, added[bestIdx])
			results = append(results, diffResult{
				kind:    "changed",
				oldLine: oldHighlighted,
				newLine: newHighlighted,
			})
			usedAdded[bestIdx] = true
		} else {
			results = append(results, diffResult{kind: "removed", line: rem})
		}
	}

	for j, add := range added {
		if !usedAdded[j] {
			results = append(results, diffResult{kind: "added", line: add})
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

				if len(results) > 0 {
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
