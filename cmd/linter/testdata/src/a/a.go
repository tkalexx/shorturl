package a

import (
	"log"
	"os"
)

func badPanic() {
	panic("boom") // want `panic is forbidden`
}

func badLogFatal() {
	log.Fatal("bye") // want `log.Fatal is forbidden outside main function of package main`
}

func badLogFatalf() {
	log.Fatalf("bye %d", 1) // want `log.Fatalf is forbidden outside main function of package main`
}

func badLogFatalln() {
	log.Fatalln("bye") // want `log.Fatalln is forbidden outside main function of package main`
}

func badOsExit() {
	os.Exit(1) // want `os.Exit is forbidden outside main function of package main`
}

func okLocalFatal() {
	fatal := func(string) {}
	fatal("not stdlib")
}
