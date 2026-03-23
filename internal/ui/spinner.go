package ui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

var brailleFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner starts a terminal spinner with the given message and returns a stop function.
// When stderr is not a TTY or color is disabled (NO_COLOR / --no-color), it prints
// the message once and returns a no-op stop function.
func Spinner(msg string) func() {
	if color.NoColor || !isatty.IsTerminal(os.Stderr.Fd()) {
		fmt.Fprintln(os.Stderr, msg)
		return func() {}
	}

	done := make(chan struct{})
	var once sync.Once

	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r%s %s", brailleFrames[i%len(brailleFrames)], msg)
				i++
			}
		}
	}()

	stop := func() {
		once.Do(func() {
			close(done)
			fmt.Fprint(os.Stderr, "\r\033[K")
		})
	}
	return stop
}
