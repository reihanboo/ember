package ui

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

func ReadKey(input *os.File) (key rune, read bool, err error) {
	if input == nil {
		return 0, false, errors.New("keyboard input is nil")
	}
	fd := int(input.Fd())
	if !term.IsTerminal(fd) {
		return 0, false, nil
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return 0, false, fmt.Errorf("set terminal raw mode: %w", err)
	}
	defer func() {
		if restoreErr := term.Restore(fd, state); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore terminal mode: %w", restoreErr))
		}
	}()
	var inputByte [1]byte
	if _, err := io.ReadFull(input, inputByte[:]); err != nil {
		return 0, false, fmt.Errorf("read keyboard input: %w", err)
	}
	return rune(inputByte[0]), true, nil
}
