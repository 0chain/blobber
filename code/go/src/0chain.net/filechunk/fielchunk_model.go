package filechunk

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"

	"math"

	"os"
	"path/filepath"
	"strconv"

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
		// fmt.Println("Creating", outfn)
		out[i], err = os.Create(filepath.Join(dir, outfn))
		// fmt.Println("out[i]", out)
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

	for i := range data {
		f, err := os.Open(out[i].Name())
		checkErr(err)
		input[i] = f
		defer f.Close()
		const fileChunk = 1 * (1 << 16)
		fmt.Println("main file size", instat.Size())
		// outfurther := make([]byte, BufferSize)
		ffn, err := f.Stat()
		checkErr(err)
		fileSize := int(ffn.Size())
		fmt.Println("small file size", fileSize)
		totalPartsNum := int(math.Ceil(float64(fileSize) / float64(fileChunk)))
		// chunksizes := make([]Chunks, totalPartsNum)
		fmt.Println("totalPartsNum", totalPartsNum)
		for j := 0; j < totalPartsNum; j++ {
			partSize := int(math.Min(fileChunk, float64(fileSize-j*fileChunk)))
			partBuffer := make([]byte, partSize)
			f.Read(partBuffer)
			fileName1 := "sampleFiles" + strconv.Itoa(i) + strconv.Itoa(j)
			// target_url := "http://localhost:5050/v1/file/upload/sampleTransaction"
			// postFile(fileName1, target_url)
			_, err := os.Create(fileName1)

			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			ioutil.WriteFile(fileName1, partBuffer, os.ModeAppend)

			// fmt.Println("Split to : ", fileName1)
			target_url := "http://localhost:5050/v1/file/upload/sampleTransaction"
			postFile(fileName1, target_url)
		}
		out[i].Close()
		fin, err := os.Open(out[i].Name())
		checkErr(err)
		input[i] = fin
		defer fin.Close()
	}

	// Create parity output writers
	parity := make([]io.Writer, fi.ParShards)
	fmt.Println("parity", parity)
	for i := range parity {
		parity[i] = out[fi.DataShards+i]
		defer out[fi.DataShards+i].Close()
	}

	// Encode parity
	err = enc.Encode(input, parity)
	checkErr(err)
	fmt.Printf("File split into %d data + %d parity shards.\n", fi.DataShards, fi.ParShards)
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(2)
	}
}

func postFile(filename string, targetUrl string) error {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	fmt.Println("body buffer", bodyBuf)
	// Logger.Info("body buffer", zap.Any("body", bodyBuf))

	// this step is very important
	fileWriter, err := bodyWriter.CreateFormFile("uploadFile", filename)
	if err != nil {
		return err
	}

	// open file handle
	fh, err := os.Open("./" + filename)
	if err != nil {
		return err
	}
	defer fh.Close()

	//iocopy
	_, err = io.Copy(fileWriter, fh)
	if err != nil {
		return err
	}

	bodyWriter.WriteField("uploadDirPath", "testDir")

	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	resp, err := http.Post(targetUrl, contentType, bodyBuf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Println(resp_body)
	return nil
}
