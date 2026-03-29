package rename

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"dangernoodle.io/terranoodle/internal/output"
)

// ConfirmCandidates presents each Candidate interactively and returns
// the user-confirmed RenamePairs. Creates that are claimed by one destroy
// are removed from subsequent prompts. When autoConfirm is true and there is
// exactly one available create, the rename is auto-confirmed without reading stdin.
func ConfirmCandidates(r io.Reader, w io.Writer, candidates []Candidate, autoConfirm bool) ([]RenamePair, error) {
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
			if autoConfirm {
				fmt.Fprintf(w, "%s %s -> %s\n", output.Bold("Auto-confirmed:"), output.Cyan("%s", c.Destroy.Address), output.Cyan("%s", available[0]))
				pairs = append(pairs, RenamePair{From: c.Destroy.Address, To: available[0]})
				consumed[available[0]] = true
			} else {
				fmt.Fprintf(w, "\nRename %s -> %s? [y/N] ", c.Destroy.Address, available[0])
				if !scanner.Scan() {
					break
				}
				answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
				if answer == "y" || answer == "yes" {
					pairs = append(pairs, RenamePair{From: c.Destroy.Address, To: available[0]})
					consumed[available[0]] = true
				}
			}
		} else {
			if autoConfirm {
				fmt.Fprintf(w, "\n%s %s could be renamed to:\n", output.Bold("Ambiguous:"), c.Destroy.Address)
				for i, addr := range available {
					fmt.Fprintf(w, "  [%d] %s\n", i+1, addr)
				}
				fmt.Fprintf(w, "  (use --apply to choose interactively)\n")
				continue
			}
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
