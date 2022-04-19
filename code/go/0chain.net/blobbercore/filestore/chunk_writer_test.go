//go:build !integration
// +build !integration

package filestore

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite(t *testing.T) {
	fileName := filepath.Join(os.TempDir(), "testwrite_"+strconv.FormatInt(time.Now().Unix(), 10))

	content := "this is full content"

	w, err := NewChunkWriter(fileName)
	if err != nil {
		require.Error(t, err, "failed to create ChunkWriter")
		return
	}

	_, err = w.Write([]byte(content))

	if err != nil {
		require.Error(t, err, "failed to ChunkWriter.WriteChunk")
		return
	}

	buf := make([]byte, w.Size())

	//read all lines from file
	_, err = w.Read(buf)
	if err != nil {
		require.Error(t, err, "failed to ChunkWriter.Read")
		return
	}

	assert.Equal(t, content, string(buf), "File content should be same")
}

func TestWriteChunk(t *testing.T) {
	chunk1 := "this is 1st chunked"

	tempFile, err := ioutil.TempFile("", "")

	if err != nil {
		require.Error(t, err, "failed to create tempfile")
		return
	}
	offset, err := tempFile.WriteString(chunk1)
	if err != nil {
		require.Error(t, err, "failed to write first chunk to tempfile")
		return
	}

	fileName := tempFile.Name()
	tempFile.Close()

	w, err := NewChunkWriter(fileName)
	if err != nil {
		require.Error(t, err, "failed to create ChunkWriter")
		return
	}
	defer w.Close()

	chunk2 := "this is 2nd chunk"

	_, err = w.WriteChunk(context.TODO(), int64(offset), strings.NewReader(chunk2))

	if err != nil {
		require.Error(t, err, "failed to ChunkWriter.WriteChunk")
		return
	}

	buf := make([]byte, w.Size())

	//read all lines from file
	_, err = w.Read(buf)
	if err != nil {
		require.Error(t, err, "failed to ChunkWriter.Read")
		return
	}

	assert.Equal(t, chunk1+chunk2, string(buf), "File content should be same")
}
