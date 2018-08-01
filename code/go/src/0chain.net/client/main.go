package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"

	"go.uber.org/zap"

	"0chain.net/logging"
	. "0chain.net/logging"
)

func postFile(filename string, targetUrl string) error {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// this step is very important
	fileWriter, err := bodyWriter.CreateFormFile("uploadFile", filename)
	if err != nil {
		Logger.Info("error writing to buffer")
		return err
	}

	// open file handle
	fh, err := os.Open("./" + filename)
	if err != nil {
		Logger.Info("error opening file")
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
	Logger.Info(resp.Status)
	Logger.Debug("response body", zap.Any("resp_body", resp_body))
	return nil
}

// sample usage
func main() {
	logging.InitLogging("development")
	target_url := "http://localhost:5050/v1/file/upload/sampleTransaction"
	filename := "test.txt"
	postFile(filename, target_url)
}
