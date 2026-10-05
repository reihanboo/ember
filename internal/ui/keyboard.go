package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/term"
)

type KeyEvent uint8

const (
	ReloadKey KeyEvent = iota + 1
	ToggleKey
	ClearKey
	QuitKey
)

func KeyEventForByte(key byte) (KeyEvent, bool) {
	switch key {
	case 'r':
		return ReloadKey, true
	case 's':
		return ToggleKey, true
	case 'c':
		return ClearKey, true
	case 'q', 3:
		return QuitKey, true
	default:
		return 0, false
	}
}

func ClearScreen(output io.Writer) error {
	file, ok := output.(*os.File)
	if !ok || file == nil || !term.IsTerminal(int(file.Fd())) {
		return nil
	}
	if _, err := io.WriteString(file, "\x1b[2J\x1b[H"); err != nil {
		return fmt.Errorf("clear terminal: %w", err)
	}
	return nil
}

func ReadKey(input *os.File) (rune, bool, error) {
	return ReadKeyContext(context.Background(), input)
}

func ReadKeyContext(ctx context.Context, input *os.File) (key rune, read bool, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if input == nil {
		return 0, false, errors.New("keyboard input is nil")
	}
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	fd := int(input.Fd())
	if !term.IsTerminal(fd) {
		return 0, false, nil
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return 0, false, fmt.Errorf("set terminal raw mode: %w", err)
	}
	var restoreOnce sync.Once
	var restoreErr error
	restore := func() {
		restoreOnce.Do(func() {
			restoreErr = term.Restore(fd, state)
		})
	}
	stopRestore := context.AfterFunc(ctx, restore)
	defer func() {
		stopRestore()
		restore()
		if restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore terminal mode: %w", restoreErr))
		}
	}()
	var inputByte [1]byte
	if _, err := io.ReadFull(input, inputByte[:]); err != nil {
		return 0, false, fmt.Errorf("read keyboard input: %w", err)
	}
	return rune(inputByte[0]), true, nil
}
