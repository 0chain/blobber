package encoder

import (

)

type StorageSDK interface {
	AddBlobber(url string, id string, txnHash string)
	EncodeAndUpload(filepath string) (error)
	Download(path string) (error)
	List(path string) (error)
}


type Blobber struct {
	URL string
	ID string
	TxnHash string
	UploadURL string
	MetaURL string
	DownloadURL string
	ListURL string
}


