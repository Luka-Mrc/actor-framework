package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lukam/actor-framework/internal/app"
)

func main() {
	mode := flag.String("mode", app.Env("MODE", "provider"), "provider|p2p")
	role := flag.String("role", app.Env("ROLE", "coordinator"), "coordinator|trainer (samo provider)")
	flag.Parse()

	switch *mode {
	case "p2p":
		app.RunPeer()
	case "provider":
		switch *role {
		case "trainer":
			app.RunTrainer()
		case "coordinator":
			app.RunCoordinator()
		default:
			fmt.Fprintf(os.Stderr, "nepoznat --role=%q (koristi coordinator|trainer)\n", *role)
			os.Exit(2)
		}
	default:
		fmt.Fprintf(os.Stderr, "nepoznat --mode=%q (koristi provider|p2p)\n", *mode)
		os.Exit(2)
	}
}
