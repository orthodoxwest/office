// Command office renders the hours of the Benedictine Office as used by the
// AWRV: a text ordo, plain-text and LaTeX hours, data audits, review tooling,
// and the web server. The commands themselves live in internal/cli.
package main

import (
	"os"
	_ "time/tzdata"

	"github.com/orthodoxwest/office/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
