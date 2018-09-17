package sdk

import (
	"os"
	"net/url"
	"net/http"
	"encoding/json"
	storagesdk "0chain.net/encoder"
)


type listResponseEntity struct {
	Name string `json:"name"`
	LookupHash string `"json:"lookup_hash"`
	IsDir bool `json:"is_dir"`
}

type listResponse struct {
	ListEntries []listResponseEntity `json:"entries"`
}

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
	blobber.ListURL = url + "/v1/file/list/" + enc.allocationID
	enc.blobberList = append(enc.blobberList, blobber)
}

func (enc *StorageErasureEncoder) EncodeAndUpload(filepath string) (error) {
	return enc.encoder.EncodeAndUpload(filepath, enc.blobberList)
}

func (enc *StorageErasureEncoder) DownloadAndDecode(filepath string, destFilePath string) (error) {
	
	f, err := os.Create(destFilePath)
	defer f.Close()
	if(err != nil) {
		return err
	}

	err = enc.encoder.DownloadAndDecode(filepath, enc.blobberList, f)
	return err
}

func (enc *StorageErasureEncoder) ListEntities(filepath string) ([]byte, error) {
	//return enc.encoder.ListEntities(filepath, enc.blobberList)
	listEntries := make([]listResponseEntity, 0)
	for i:= range enc.blobberList {
		u, _ := url.Parse(enc.blobberList[i].ListURL)
		q := u.Query()
		q.Set("path", filepath)
		u.RawQuery = q.Encode()
		
		var listResponseObj listResponse
		listResponseObj.ListEntries = listEntries
		response, err := http.Get(u.String())
	    if err != nil {
	        return nil, err
	    } else {
	        defer response.Body.Close()

	        err = json.NewDecoder(response.Body).Decode(&listResponseObj)
	        if(err != nil) {
	        	return nil, err
	        }
	        
	    }
	}
	return nil, nil

}
