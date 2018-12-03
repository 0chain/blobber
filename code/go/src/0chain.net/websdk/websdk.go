package main

import (
	"fmt"
	"bytes"
	"bufio"
	"github.com/klauspost/reedsolomon"
	"reflect"
	"syscall/js"
	"unsafe"
)

type ZCNStreamEncoder struct {
	encoder      	reedsolomon.Encoder
	dataShards   	int
	parityShards 	int
	setupCb 		js.Callback
	encodeCb		js.Callback
	decodeCb		js.Callback
	unloadCh 		chan struct {}
	unloadCb		js.Callback
}

const (
	DATA_SHARDS_DEFAULT   = 10
	PARITY_SHARDS_DEFAULT = 3
)

var WebSDK ZCNStreamEncoder

func init() {
	WebSDK.dataShards 		= 	DATA_SHARDS_DEFAULT
	WebSDK.parityShards 	= 	PARITY_SHARDS_DEFAULT
	WebSDK.setupCb			=	js.NewCallback(ZCNWebSdkSetup)
	WebSDK.encodeCb			=	js.NewCallback(ZCNWebSdkEncode)
	WebSDK.decodeCb			= 	js.NewCallback(ZCNWebSdkDecode)
	WebSDK.unloadCh			= 	make(chan struct{})
	WebSDK.unloadCb			= 	js.NewCallback(ZCNWebSdkUnload)
}

// args[0] : Number of data shards
// args[1] : Number of parity shards
func ZCNWebSdkSetup(args []js.Value) {
	if (len(args) < 2) {
		return
	}
	dataShards 		:= args[0].Int()
	parityShards	:= args[1].Int()
	enc, err 	:= reedsolomon.New(dataShards, parityShards)
	if err != nil {
		fmt.Println("ZCNWebSdkSetup(): ",err.Error())
		return
	}
	WebSDK.encoder 		= enc
	WebSDK.dataShards	= dataShards
	WebSDK.parityShards	= parityShards
}

// args[0] : Uint8Array JS buffer
// args[1] : Callback function for encoded data
func ZCNWebSdkEncode(args []js.Value) {
	if (len(args) < 2) {
		return
	}
	inputJsData := js.ValueOf(args[0])
	// Copy js data to go buffer
	inputData := make([]byte, inputJsData.Length())
	for i := 0; i < inputJsData.Length(); i++ {
		inputData[i] = byte(inputJsData.Index(i).Int())
	}
	
	data, err := WebSDK.encoder.Split(inputData)
	if err != nil {
		fmt.Println("ZCNWebSdkEncode(): ", err.Error())
		return
	}
	err = WebSDK.encoder.Encode(data)
	if err != nil {
		fmt.Println("ZCNWebSdkEncode(): ", err.Error)
		return
	}

	for i := 0; i < (WebSDK.dataShards + WebSDK.parityShards); i++ {
		hdr := (*reflect.SliceHeader)(unsafe.Pointer(&data[i]))
		ptr := uintptr(unsafe.Pointer(hdr.Data))
		js.Global().Call(js.ValueOf(args[1]).String(), i, len(data[i]), ptr)
	}
}

// args[0]..[x] = Number of shards (data + parity)
// args[x+1] = Callback function of joined buffer
func ZCNWebSdkDecode(args []js.Value) {
	if (len(args) < 2) {
		return
	}
	inputJsData 	:= js.ValueOf(args[0])
	numshards	  	:= inputJsData.Length()
	var inputData [][]byte
	var bytesPerShard int
	inputData 		= make([][]byte, numshards)
	for shards := 0; shards < numshards; shards++ {
		jsShard  		:= js.ValueOf(inputJsData).Index(shards)
		bytesPerShard	= js.ValueOf(jsShard).Length()
		// Copy js data to go buffer
		inputData[shards] = make([]byte, bytesPerShard)
		for i := 0; i < bytesPerShard; i++ {
			inputData[shards][i] = byte(jsShard.Index(i).Int())
		}
	}
	_, err := WebSDK.encoder.Verify(inputData)
	if err != nil {
		fmt.Println("Verification failed. Reconstructing data")
		err = WebSDK.encoder.Reconstruct(inputData)
		if err != nil {
			fmt.Println("Reconstruct failed -", err)
			return
		}
		_, err = WebSDK.encoder.Verify(inputData)
		if err != nil {
			fmt.Println("Verification failed after reconstruction, data likely corrupted.", err.Error())
			return
		}
	}
	var bytesBuf bytes.Buffer
	bufWriter := bufio.NewWriter(&bytesBuf)
	bufWriter = bufio.NewWriterSize(bufWriter, (bytesPerShard * WebSDK.dataShards))
	err = WebSDK.encoder.Join(bufWriter, inputData, (bytesPerShard * WebSDK.dataShards))
	if err != nil {
		fmt.Println("join failed", err.Error(), inputData)
		return
	}
	bufWriter.Flush()
	outBuf := bytesBuf.Bytes()
	// fmt.Println(bytesBuf.Len(), outBuf)
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&outBuf))
	ptr := uintptr(unsafe.Pointer(hdr.Data))
	js.Global().Call(js.ValueOf(args[1]).String(), len(outBuf), ptr)
}

// No argument
func ZCNWebSdkUnload(args []js.Value) {
	WebSDK.unloadCh <- struct {} {}
}


func exportFunctions() {
	js.Global().Set("ZCNWebSdkSetup", WebSDK.setupCb)
	js.Global().Set("ZCNWebSdkEncode", WebSDK.encodeCb)
	js.Global().Set("ZCNWebSdkDecode", WebSDK.decodeCb)
	js.Global().Set("ZCNWebSdkUnload", WebSDK.unloadCb)
}

func releaseFunctions() {
	WebSDK.setupCb.Release()
	WebSDK.encodeCb.Release()
	WebSDK.decodeCb.Release()
	WebSDK.unloadCb.Release()
}


func main() {
	exportFunctions()
	fmt.Println("0Chain WebSDK WASM Initialized!!")
	// Wait for beforeUnload event to cleanup resource
	<-WebSDK.unloadCh
	releaseFunctions()
	fmt.Println("0Chain WebSDK WASM Uninitialized!!")
}
