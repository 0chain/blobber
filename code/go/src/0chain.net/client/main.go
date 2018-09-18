package main

import (
	"fmt"
	"os"
	"runtime"

	"0chain.net/sdk"
)

// sample usage
func main() {

	runtime.GOMAXPROCS(30)

	fname := "big.txt"

	enc := sdk.NewErasureEncoder("36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80", 4, 2)
	enc.AddBlobber("http://localhost:5051", "ccde4b06d02e24113889164d94a9692284f60c8701f10e600bde58889d335055", "36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")
	enc.AddBlobber("http://localhost:5052", "c8a282a1ab321c083b92f206cafbf6f06f6a38f5764257e2456ddfa38cb0fb6b", "36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")
	enc.AddBlobber("http://localhost:5053", "ce9f720e8d8082cf4946575879de0630b53efc52e522ce1f8cb85bb0928f7b83", "36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")
	enc.AddBlobber("http://localhost:5054", "cef4e233a3c3dee37ec3c293bf93bde0aa59876ca2d7cc895f90eb2f5bf0ee1b", "36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")
	enc.AddBlobber("http://localhost:5055", "d0339f664b738fee9085b07cb80b0da522a06b4a7a659a12d792cb68aedf2e6a", "36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")
	enc.AddBlobber("http://localhost:5056", "d74b590ba2255d77bf2dc2a4f33bec6d07752730d5016ce58f6e7eaeba45af23", "36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")
	// err := enc.EncodeAndUpload(fname)
	err := enc.DownloadAndDecode("/"+fname, "big_result.txt")

	checkErr(err)

	fmt.Printf("File split into %d data + %d parity shards.\n", 10, 6)
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(2)
	}
}
