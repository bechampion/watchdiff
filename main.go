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
	red    = "\033[1;91m"
	green  = "\033[1;92m"
	cyan   = "\033[1;96m"
	dim    = "\033[2;37m"
	bold   = "\033[1;97m"
	yellow = "\033[1;93m"
	reset  = "\033[0m"
)

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func run(command string) string {
	cmd := exec.Command("sh", "-c", command)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func diff(prev, curr []string) (added, removed []string) {
	old := make(map[string]int)
	for _, line := range prev {
		old[line]++
	}
	new := make(map[string]int)
	for _, line := range curr {
		new[line]++
	}

	for _, line := range prev {
		if new[line] < old[line] {
			removed = append(removed, line)
			old[line]--
		} else {
			old[line]--
		}
	}

	oldReset := make(map[string]int)
	for _, line := range prev {
		oldReset[line]++
	}
	for _, line := range curr {
		if oldReset[line] <= 0 {
			added = append(added, line)
		} else {
			oldReset[line]--
		}
	}

	return added, removed
}

func lines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: watchdiff [-n seconds] command\n")
	fmt.Fprintf(os.Stderr, "\nRuns a command repeatedly and shows colored diff output.\n")
	fmt.Fprintf(os.Stderr, "  %s- removed%s\n", red, reset)
	fmt.Fprintf(os.Stderr, "  %s+ added%s\n", green, reset)
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

	fmt.Printf("👀 %swatchdiff%s %s%s%s every %s%.1fs%s\n",
		bold, reset, cyan, command, reset, yellow, interval, reset)

	var prev []string
	tick := time.Duration(interval * float64(time.Second))
	frame := 0

	spinTick := time.NewTicker(80 * time.Millisecond)
	cmdTick := time.NewTicker(tick)
	defer spinTick.Stop()
	defer cmdTick.Stop()

	out := run(command)
	prev = lines(out)
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

		case <-cmdTick.C:
			out := run(command)
			curr := lines(out)

			if prev != nil {
				added, removed := diff(prev, curr)

				if len(added) > 0 || len(removed) > 0 {
					fmt.Printf("\r\033[K")

					now := time.Now().Format("15:04:05")
					fmt.Printf("%s🕐 %s%s\n", dim, now, reset)

					for _, line := range removed {
						fmt.Printf("  %s🔴 - %s%s\n", red, line, reset)
					}
					for _, line := range added {
						fmt.Printf("  %s🟢 + %s%s\n", green, line, reset)
					}

					fmt.Println()
				}
			}

			prev = curr
		}
	}
}
