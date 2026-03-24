package rename

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ConfirmCandidates presents each Candidate interactively and returns
// the user-confirmed RenamePairs. Creates that are claimed by one destroy
// are removed from subsequent prompts.
func ConfirmCandidates(r io.Reader, w io.Writer, candidates []Candidate) ([]RenamePair, error) {
	scanner := bufio.NewScanner(r)
	consumed := make(map[string]bool)
	var pairs []RenamePair

	for _, c := range candidates {
		// Filter out already-consumed creates.
		var available []string
		for _, cr := range c.Creates {
			if !consumed[cr.Address] {
				available = append(available, cr.Address)
			}
		}
		if len(available) == 0 {
			continue
		}

		if len(available) == 1 {
			fmt.Fprintf(w, "\nRename %s -> %s? [y/N] ", c.Destroy.Address, available[0])
			if !scanner.Scan() {
				break
			}
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer == "y" || answer == "yes" {
				pairs = append(pairs, RenamePair{From: c.Destroy.Address, To: available[0]})
				consumed[available[0]] = true
			}
		} else {
			fmt.Fprintf(w, "\nWhich resource is the rename of %s?\n", c.Destroy.Address)
			for i, addr := range available {
				fmt.Fprintf(w, "  [%d] %s\n", i+1, addr)
			}
			fmt.Fprintf(w, "  [s] skip\n")
			fmt.Fprintf(w, "Choice: ")
			if !scanner.Scan() {
				break
			}
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer == "s" || answer == "skip" || answer == "" {
				continue
			}
			idx, err := strconv.Atoi(answer)
			if err != nil || idx < 1 || idx > len(available) {
				fmt.Fprintf(w, "  skipping (invalid choice)\n")
				continue
			}
			selected := available[idx-1]
			pairs = append(pairs, RenamePair{From: c.Destroy.Address, To: selected})
			consumed[selected] = true
		}
	}

	return pairs, nil
}
