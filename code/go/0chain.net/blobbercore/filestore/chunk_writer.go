package filestore

import (
	"context"
	"errors"
	"io"
	"os"
)

// ChunkWriter implements a chunk write that will append content to the file
type ChunkWriter struct {
	file   string
	writer *os.File
	reader *os.File
	offset int64
	size   int64
}

// NewChunkWriter create a ChunkWriter
func NewChunkWriter(file string) (*ChunkWriter, error) {
	w := &ChunkWriter{
		file: file,
	}
	var f *os.File
	fi, err := os.Stat(file)
	if errors.Is(err, os.ErrNotExist) {
		f, err = os.Create(file)
		if err != nil {
			return nil, err
		}
	} else {
		f, err = os.OpenFile(file, os.O_RDONLY|os.O_CREATE|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			return nil, err
		}

		w.size = fi.Size()
		w.offset = fi.Size()
	}

	w.writer = f

	return w, nil
}

//Write implements io.Writer
func (w *ChunkWriter) Write(b []byte) (n int, err error) {
	if w == nil || w.writer == nil {
		return 0, os.ErrNotExist
	}

	written, err := w.writer.Write(b)

	w.size += int64(written)

	return written, err
}

//Reader implements io.Reader
func (w *ChunkWriter) Read(p []byte) (n int, err error) {
	if w == nil || w.reader == nil {
		reader, err := os.Open(w.file)

		if err != nil {
			return 0, err
		}

		w.reader = reader
	}

	return w.reader.Read(p)
}

//WriteChunk append data to the file
func (w *ChunkWriter) WriteChunk(ctx context.Context, offset int64, src io.Reader) (int64, error) {
	if w == nil || w.writer == nil {
		return 0, os.ErrNotExist
	}

	_, err := w.writer.Seek(offset, io.SeekStart)

	if err != nil {
		return 0, err
	}

	n, err := io.Copy(w.writer, src)

	w.offset += n
	w.size += n

	return n, err
}

//Size length in bytes for regular files
func (w *ChunkWriter) Size() int64 {
	if w == nil {
		return 0
	}
	return w.size
}

//Close closes the underline File
func (w *ChunkWriter) Close() {
	if w == nil {
		return
	}

	if w.writer != nil {
		w.writer.Close()
	}

	if w.reader != nil {
		w.reader.Close()
	}
}
