package proc

import (
	"bytes"
	"io"
	"sync"
)

type lineWriter struct {
	writer   io.Writer
	prefix   []byte
	outputMu *sync.Mutex
	mu       sync.Mutex
	pending  []byte
}

func newLineWriter(writer io.Writer, tag string, outputMu *sync.Mutex) *lineWriter {
	return &lineWriter{
		writer:   writer,
		prefix:   []byte("[" + tag + "] "),
		outputMu: outputMu,
	}
}

func (w *lineWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	written := len(data)
	w.pending = append(w.pending, data...)
	for {
		newline := bytes.IndexByte(w.pending, '\n')
		if newline < 0 {
			break
		}
		if err := w.writeLine(w.pending[:newline+1]); err != nil {
			w.pending = nil
			return written, err
		}
		w.pending = w.pending[newline+1:]
	}
	return written, nil
}

func (w *lineWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) == 0 {
		return nil
	}
	return w.writeLine(w.pending)
}

func (w *lineWriter) writeLine(line []byte) error {
	record := make([]byte, 0, len(w.prefix)+len(line)+1)
	record = append(record, w.prefix...)
	record = append(record, line...)
	if len(line) == 0 || line[len(line)-1] != '\n' {
		record = append(record, '\n')
	}

	w.outputMu.Lock()
	defer w.outputMu.Unlock()
	for len(record) > 0 {
		written, err := w.writer.Write(record)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		record = record[written:]
	}
	return nil
}
