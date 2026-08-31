package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// interactive reports whether we may prompt: stdin must be a real terminal.
//
// This is the contract that keeps scripts, CI, and the agent skill working.
// A non-TTY never prompts and never blocks — it gets the error it always got.
func interactive(cmd *cobra.Command) bool {
	f, ok := cmd.InOrStdin().(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// choice is one option offered by pick.
type choice struct {
	ID    string
	Label string
}

// pick asks the user to choose from candidates, returning the chosen ID.
// Callers must check interactive() first.
func pick(cmd *cobra.Command, header string, candidates []choice) (string, error) {
	w := cmd.ErrOrStderr()
	fmt.Fprintln(w, header)
	width := len(strconv.Itoa(len(candidates)))
	for i, c := range candidates {
		fmt.Fprintf(w, "  %*d) %s  %s\n", width, i+1, c.Label, c.ID)
	}
	fmt.Fprintf(w, "Pick [1-%d, or q to cancel]: ", len(candidates))

	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read choice: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" || strings.EqualFold(line, "q") {
		return "", fmt.Errorf("cancelled")
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(candidates) {
		return "", fmt.Errorf("%q is not one of 1-%d", line, len(candidates))
	}
	return candidates[n-1].ID, nil
}
