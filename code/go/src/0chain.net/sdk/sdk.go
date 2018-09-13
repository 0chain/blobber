package sdk

import (
	"os"
	storagesdk "0chain.net/encoder"
)

type StorageErasureEncoder struct {
	encoder storagesdk.ReedsolomonStreamEncoder
	allocationID string
	blobberList []storagesdk.Blobber
}

func NewErasureEncoder(allocationID string, dataShards int, parityShards int) (*StorageErasureEncoder){
	enc := &StorageErasureEncoder{allocationID: allocationID}
	enc.blobberList = make([]storagesdk.Blobber, 0)
	enc.encoder.Init(dataShards, parityShards)
	return enc
}


func (enc *StorageErasureEncoder) AddBlobber(url string, id string, txnHash string) {
	var blobber storagesdk.Blobber
	blobber.URL = url
	blobber.ID = id
	blobber.TxnHash = txnHash
	blobber.UploadURL = url + "/v1/file/upload/" + enc.allocationID
	blobber.MetaURL = url + "/v1/file/meta/" + enc.allocationID
	blobber.DownloadURL = url + "/v1/file/download/" + enc.allocationID
	enc.blobberList = append(enc.blobberList, blobber)
}

func (enc *StorageErasureEncoder) EncodeAndUpload(filepath string) (error) {
	return enc.encoder.EncodeAndUpload(filepath, enc.blobberList)
}

func (enc *StorageErasureEncoder) DownloadAndDecode(filepath string) (error) {
	// Join the shards and write them
	outfn := "big_result.txt"
	
	f, err := os.Create(outfn)
	defer f.Close()
	if(err != nil) {
		return err
	}

	err = enc.encoder.DownloadAndDecode(filepath, enc.blobberList, f)
	return err
}
