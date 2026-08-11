package main

import (
	"log"
	"os"
)

func main() {
	// разрешено внутри main пакета main
	log.Fatal("ok in main")
	os.Exit(0)
}

func helper() {
	panic("no")    // want `panic is forbidden`
	log.Fatal("x") // want `log.Fatal is forbidden outside main function of package main`
	os.Exit(2)     // want `os.Exit is forbidden outside main function of package main`
}
