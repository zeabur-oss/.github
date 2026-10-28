package main

import (
	"io"
	"os"

	"zeabur-oss-release/internal/notes"
)

func notesCommand(args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return usageError("notes takes <changelog> <version>")
	}
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()
	text, err := notes.Section(f, args[1])
	if err != nil {
		return err
	}
	_, err = io.WriteString(stdout, text)
	return err
}
