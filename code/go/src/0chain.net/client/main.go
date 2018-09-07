package main

import (
	"fmt"
	"os"
	"mime/multipart"
	"io/ioutil"
	"net/http"
	"io"
	"sync"
	"github.com/klauspost/reedsolomon"
	"runtime"
)

func uploadFile(filename string, reader io.Reader, wg *sync.WaitGroup) error {
	defer wg.Done()
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

		//iocopy
		_, err = io.Copy(fileWriter, reader)
		if err != nil {
			bodyWriter.CloseWithError(err)
			return
		}

		bodyWriter.CloseWithError(multiWriter.Close())
	}()
	contentType := multiWriter.FormDataContentType()
	targetUrl := "http://localhost:5050/v1/file/upload/36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80"
	resp, err := http.Post(targetUrl, contentType, bodyReader)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err

	}
	fmt.Println("resp", string(resp_body))
	return nil
}

func storeInFile(in io.Reader, i int, wg *sync.WaitGroup) {
	defer wg.Done()
	destfilename := fmt.Sprintf("%s.%d", "big.txt", i)
	fmt.Println("file to be created" , destfilename)
	f, err := os.Create(destfilename)
	defer f.Close()
	checkErr(err)
	// copy from reader data into writer file
	_, err = io.Copy(f,in)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("file created" , destfilename)
}

// sample usage
func main() {

	runtime.GOMAXPROCS(30)

	fname:="big.txt"
	// Create encoding matrix.
	enc, err := reedsolomon.NewStreamC(10, 6, true, true)
	checkErr(err)

	fmt.Println("Opening", fname)
	f, err := os.Open(fname)
	checkErr(err)

	instat, err := f.Stat()
	checkErr(err)

	shards := 10 

	var wg sync.WaitGroup
	//wg.Add(12)
	wg.Add(18)

	out1 := make([]io.Writer, shards)
	out2 := make([]io.Writer, shards)
	out := make([]io.Writer, shards)

	in := make([]io.Reader, shards)
	inr := make([]io.Reader, shards)
	for i := range out {
		outfn := fmt.Sprintf("Part : %d", i)
		fmt.Println("Creating", outfn)
		pr, pw := io.Pipe()
		npr, npw := io.Pipe()
		out1[i] = pw
		out2[i] = npw
		out[i] = io.MultiWriter(pw, npw)
		//out[i] = pw
		checkErr(err)
		//tr := io.TeeReader(pr, f)
		in[i] = pr
		inr[i] = npr
		destfilename := fmt.Sprintf("%s.%d", "big.txt", i)
		go uploadFile(destfilename, npr, &wg)
		//go storeInFile(npr, i, &wg);
	}

	

	// Create parity output writers
	parity := make([]io.Writer, 6)
	for i := range parity {
		// destfilename := fmt.Sprintf("%s.%d", "big.txt", 10+i)
		// fmt.Println("file to be created" , destfilename)
		// f, err := os.Create(destfilename)
		// defer f.Close()
		// checkErr(err)
		// parity[i] = f
		// //parity[i] = out[10+i]
		// //defer out[10+i].(*io.PipeWriter).Close()
		// fmt.Println("file created" , destfilename)
		pr, pw := io.Pipe()
		parity[i] = pw
		destfilename := fmt.Sprintf("%s.%d", "big.txt", 10+i)
		go uploadFile(destfilename, pr, &wg)

	}

	go func() {
		defer wg.Done()
		// Encode parity
		err = enc.Encode(in, parity)
		checkErr(err)	
		for i:= range parity {
			parity[i].(*io.PipeWriter).Close()
		}
	}()

	go func() {
		defer wg.Done()
		// Do the split
		err = enc.Split(f, out, instat.Size())
		checkErr(err)
		fmt.Println("Done with split")
		for i := range out {
			out2[i].(*io.PipeWriter).Close()
			out1[i].(*io.PipeWriter).Close()
			//out2[i].(*io.PipeWriter).Close()
			//out[i].(*io.PipeWriter).Close()
			
		}
	}()

	wg.Wait()	

	fmt.Printf("File split into %d data + %d parity shards.\n", 10, 6)
}


func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(2)
	}
}