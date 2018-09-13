package filechunk

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"

	"github.com/klauspost/reedsolomon"
)

func getMeta(body []byte) (*Maininfo, error) {
	var s = new(Maininfo)
	err := json.Unmarshal(body, &s)
	if err != nil {
		fmt.Println(err)
	}
	return s, err
}

func (cm *CustomMeta) UnmarshalJSON(bs []byte) error {
	// Unquote the source string so we can unmarshal it.
	unquoted, err := strconv.Unquote(string(bs))
	if err != nil {
		return err
	}

	// Create an aliased type so we can use the default unmarshaler.
	type CustomMeta2 CustomMeta
	var cm2 CustomMeta2

	// Unmarshal the unquoted string and assign to the original object.
	if err := json.Unmarshal([]byte(unquoted), &cm2); err != nil {
		return err
	}
	*cm = CustomMeta(cm2)
	return nil
}

var FileNames []*os.File

func get(downloadURL string) ([]byte, error) {
	res, err := http.Get(downloadURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func Download() {
	targetURL := "http://localhost:5050/v1/file/download/36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80"
	downloadURL := "http://localhost:5050/v1/file/meta/36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80?path=/&filename=big.txt"

	body, err := get(downloadURL)
	if err != nil {
		panic(err)
	}

	var doc Maininfo
	errs := json.Unmarshal([]byte(body), &doc)
	if errs != nil {
		panic(err)
	}

	sort.Slice(doc.Meta, func(i, j int) bool {
		p1 := doc.Meta[i].Custom.PartNum
		p2 := doc.Meta[j].Custom.PartNum
		return p1 < p2
	})
	// for _, m := range doc.Meta {
	// 	fmt.Println(m.Custom.PartNum)
	// }

	for j := range doc.Meta {
		file := doc.Meta[j].Filename
		hash := doc.Meta[j].Content_hash
		downloadFile(file, "/", targetURL, hash)
	}

	var fileinfo = &FileInfo{DataShards: 10,
		ParShards: 6,
		OutDir:    "",
		File:      "big.txt"}
	fileinfo.DecodeFileByShards()
}

func downloadFile(filename, filepath, targetURL, part_hash string) (string, error) {
	url := targetURL + "?path=" + "/" + "&filename=" + filename + "&part_hash=" + part_hash
	fmt.Println("complete url ", url)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
	}
	// defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "false", err
	}

	file, _ := os.Create("file" + part_hash + ".txt")
	FileNames = append(FileNames, file)
	io.Copy(file, resp.Body)

	defer file.Close()
	return "true", err

}

func (fi *FileInfo) DecodeFileByShards() {

	fname := fi.File

	// Create matrix
	enc, err := reedsolomon.NewStream(fi.DataShards, fi.ParShards)
	checkErr(err)

	// Open the inputs
	shards, size, err := openInput(fi.DataShards, fi.ParShards, fname)
	checkErr(err)

	// Verify the shards
	ok, err := enc.Verify(shards)
	// fmt.Println("verify", ok)
	if ok {
		fmt.Println("No reconstruction needed")
	} else {
		fmt.Println("Verification failed. Reconstructing data")
		shards, size, err = openInput(fi.DataShards, fi.ParShards, fname)
		checkErr(err)
		// Create out destination writers
		out := make([]io.Writer, len(shards))
		for i := range out {

			if shards[i] == nil {
				outfn := fmt.Sprintf("%s", fname)
				fmt.Println("Creating", outfn)

				out[i], err = os.Create(outfn)
				checkErr(err)

			}
		}
		err = enc.Reconstruct(shards, out)
		if err != nil {
			fmt.Println("Reconstruct failed -", err)
			os.Exit(1)
		}
		// Close output.
		for i := range out {
			if out[i] != nil {
				err := out[i].(*os.File).Close()
				checkErr(err)
			}
		}

		shards, size, err = openInput(fi.DataShards, fi.ParShards, fname)
		ok, err = enc.Verify(shards)
		fmt.Println("ok", ok)
		if !ok {
			fmt.Println("Verification failed after reconstruction, data likely corrupted:", err)
			os.Exit(1)
		}
		checkErr(err)
	}

	// Join the shards and write them
	outfn := fi.OutDir
	if outfn == "" {
		outfn = fname
	}

	fmt.Println("Writing data to", outfn)
	f, err := os.Create(outfn)
	checkErr(err)

	shards, size, err = openInput(fi.DataShards, fi.ParShards, fname)
	checkErr(err)

	err = enc.Join(f, shards, int64(fi.DataShards)*size)
	checkErr(err)
}

func openInput(dataShards, parShards int, fname string) (r []io.Reader, size int64, err error) {
	// Create shards and load the data.
	shards := make([]io.Reader, dataShards+parShards)
	for i, file := range FileNames {
		// ofn := fmt.Sprintf("%d", i+1)

		f, err := os.Open(file.Name())
		if err != nil {
			fmt.Println("Error reading filesssssss", err)
			shards[i] = nil
			continue
		} else {
			shards[i] = f
		}
		stat, err := f.Stat()
		checkErr(err)
		if stat.Size() > 0 {
			size = stat.Size()
		} else {
			shards[i] = nil
		}

	}

	return shards, size, nil

}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(2)
	}
}


