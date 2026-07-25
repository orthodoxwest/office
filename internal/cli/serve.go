package cli

import (
	"fmt"

	"github.com/orthodoxwest/office/internal/web"
)

// cmdServe starts the web server and blocks.
func cmdServe(e env, args []string) error {
	addr := ":8080"
	if len(args) >= 1 {
		addr = args[0]
	}

	srv, err := web.New(e.dataDir, addr)
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}

	fmt.Fprintf(e.err, "Listening on http://localhost%s\n", addr)
	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}
