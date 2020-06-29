package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/SafetyCulture/djinni-parser/pkg/parser"
)

func usage() {
	log.Printf("usage: %s path/to/file.djinni | jq .", os.Args[0])
	os.Exit(-1)
}

func main() {
	if len(os.Args) != 2 {
		usage()
	}
	src := os.Args[1]
	f, err := parser.ParseFile(src, nil)
	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(f); err != nil {
		log.Println(err)
		os.Exit(-1)
	}
}
