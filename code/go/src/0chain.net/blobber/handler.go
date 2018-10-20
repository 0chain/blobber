package blobber

import (
	"encoding/json"

	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
)

const (
	AllocationTransactionHeader = "X-Allocation-Transaction"
	BlobberTransactionHeader    = "X-Blobber-Transaction"
)

var storageHandler StorageHandler

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	r.HandleFunc("/v1/file/upload/{allocation}", UploadHandler)
	r.HandleFunc("/v1/file/download/{allocation}", DownloadHandler)
	r.HandleFunc("/v1/file/meta/{allocation}", MetaHandler)
	r.HandleFunc("/v1/file/list/{allocation}", ListHandler)
	r.HandleFunc("/v1/data/challenge", ChallengeHandler)
	storageHandler = GetStorageHandler()
}

/*ChallengeHandler is the handler to respond to challenge requests*/
func ChallengeHandler(respW http.ResponseWriter, r *http.Request) {
	respW.Header().Set("Content-Type", "application/json")
	err := storageHandler.ChallengeData(r)
	if err != nil {
		respW.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(respW).Encode(err)
		return
	}

	json.NewEncoder(respW).Encode("challenge accepted")
	return
}

/*ListHandler is the handler to respond to list requests from clients*/
func ListHandler(respW http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	respW.Header().Set("Content-Type", "application/json")

	response, err := storageHandler.ListEntities(r, vars["allocation"])

	if err != nil {
		respW.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(respW).Encode(err)
		return
	}
	json.NewEncoder(respW).Encode(response)
	return
}

/*UploadHandler is the handler to respond to upload requests fro clients*/
func UploadHandler(respW http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	respW.Header().Set("Content-Type", "application/json")

	response := storageHandler.WriteFile(r, vars["allocation"])

	if response.Error != nil {
		respW.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(respW).Encode(response)
	return
}

/*MetaHandler is the handler to respond to file meta requests from clients*/
func MetaHandler(respW http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	respW.Header().Set("Content-Type", "application/json")

	response, err := storageHandler.GetFileMeta(r, vars["allocation"])

	if err != nil {
		respW.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(respW).Encode(err)
		return
	}
	json.NewEncoder(respW).Encode(response)
	return
}

/*DownloadHandler is the handler to respond to download requests from clients*/
func DownloadHandler(respW http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	response, err := storageHandler.DownloadFile(r, vars["allocation"])

	if err != nil {
		respW.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(respW).Encode(err)
		return
	}

	//Check if file exists and open
	Openfile, errN := os.Open(response.Path)
	defer Openfile.Close() //Close after function return
	if errN != nil {
		//File not found, send 404
		http.Error(respW, "File not found.", 404)
		return
	}

	//File is found, create and send the correct headers

	//Get the Content-Type of the file
	//Create a buffer to store the header of the file in
	FileHeader := make([]byte, 512)
	//Copy the headers into the FileHeader buffer
	Openfile.Read(FileHeader)
	//Get content type of file
	FileContentType := http.DetectContentType(FileHeader)

	//Get the file size
	FileStat, _ := Openfile.Stat()                     //Get info from file
	FileSize := strconv.FormatInt(FileStat.Size(), 10) //Get file size as a string

	//Send the headers
	respW.Header().Set("Content-Disposition", "attachment; filename="+response.Filename)
	respW.Header().Set("Content-Type", FileContentType)
	respW.Header().Set("Content-Length", FileSize)

	//Send the file
	//We read 512 bytes from the file already so we reset the offset back to 0
	Openfile.Seek(0, 0)
	io.Copy(respW, Openfile) //'Copy' the file to the client
	return
}
