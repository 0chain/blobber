package filechunk

import (
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"

	"os"
	"path/filepath"

	// . "0chain.net/logging"
	"github.com/klauspost/reedsolomon"
)

//FileInfo struct
type FileInfo struct {
	DataShards int
	ParShards  int
	File       string
	OutDir     string
}

// var Messages = make(chan *os.File)

//ChunkingFilebyShards is used to divide the file in chunks using erasure coding
func (fi *FileInfo) ChunkingFilebyShards() {
	if fi.DataShards > 257 {
		fmt.Fprintf(os.Stderr, "Error: Too many data shards\n")
		os.Exit(1)
	}
	fname := fi.File

	// Create encoding matrix.
	enc, err := reedsolomon.NewStream(fi.DataShards, fi.ParShards)
	checkErr(err)

	fmt.Println("Opening", fname)
	f, err := os.Open(fname)
	checkErr(err)

	instat, err := f.Stat()
	checkErr(err)

	shards := fi.DataShards + fi.ParShards
	out := make([]*os.File, shards)

	// Create the resulting files.
	dir, file := filepath.Split(fname)
	if fi.OutDir != "" {
		dir = fi.OutDir
	}

	for i := range out {
		outfn := fmt.Sprintf("%s.%d", file, i)
		out[i], err = os.Create(filepath.Join(dir, outfn))
		checkErr(err)
	}

	// Split into files.
	data := make([]io.Writer, fi.DataShards)
	for i := range data {
		data[i] = out[i]
	}
	// Do the split
	err = enc.Split(f, data, instat.Size())
	checkErr(err)

	// Close and re-open the files.
	input := make([]io.Reader, fi.DataShards)
	target_url := "http://localhost:5050/v1/file/upload/sampleTransaction"

	for i := range data {
		out[i].Close()
		f, err := os.Open(out[i].Name())
		checkErr(err)
		input[i] = f
		defer f.Close()
	}

	// Create parity output writers
	parity := make([]io.Writer, fi.ParShards)
	for i := range parity {
		parity[i] = out[fi.DataShards+i]
		defer out[fi.DataShards+i].Close()
	}

	err = enc.Encode(input, parity)
	checkErr(err)

	fmt.Printf("File split into %d data + %d parity shards.\n", fi.DataShards, fi.ParShards)

	for _, val := range out {
		postFile(val.Name(), target_url)
	}

}
func postFile(filename string, targetUrl string) error {
	bodyReader, bodyWriter := io.Pipe()
	multiWriter := multipart.NewWriter(bodyWriter)
	go func() {
		// fmt.Println("body buffer", bodyWriter)

		// this step is very important

		fileWriter, err := multiWriter.CreateFormFile("uploadFile", filename)
		if err != nil {
			bodyWriter.CloseWithError(err)
			return
		}
		fmt.Println("here")
		// open file handle
		fh, err := os.Open("./" + filename)
		if err != nil {
			bodyWriter.CloseWithError(err)
			return
		}
		defer fh.Close()

		//iocopy
		_, err = io.Copy(fileWriter, fh)
		if err != nil {
			bodyWriter.CloseWithError(err)
			return
		}

		multiWriter.WriteField("uploadDirPath", "")

		bodyWriter.CloseWithError(multiWriter.Close())
	}()
	contentType := multiWriter.FormDataContentType()
	resp, err := http.Post(targetUrl, contentType, bodyReader)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err

	}
	fmt.Println("resp", resp_body)
	return nil

}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(2)
	}
}
