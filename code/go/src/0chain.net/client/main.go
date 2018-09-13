package main

import (
	"fmt"
	"os"
	"0chain.net/sdk"
	"runtime"
)

// sample usage
func main() {

	runtime.GOMAXPROCS(30)

	fname:="big.txt"
	
	enc := sdk.NewErasureEncoder("36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80", 10, 6)
	enc.AddBlobber("http://localhost:5050", "36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80", "36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")

	//err:=enc.EncodeAndUpload(fname)
	err:=enc.DownloadAndDecode("/" + fname)

	checkErr(err)	

	fmt.Printf("File split into %d data + %d parity shards.\n", 10, 6)
}


func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(2)
	}
}