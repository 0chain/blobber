package encoder

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/klauspost/reedsolomon"
)

type MetaInfo struct {
	Filename           string `json:"filename"`
	CustomMeta         string `json:"custom_meta"`
	Size               int64  `json:"size"`
	ContentHash        string `json:"content_hash"`
	customMetaPartInfo PartMeta
	blobber            Blobber
}

type FileMeta struct {
	ID   string     `json:"id"`
	Meta []MetaInfo `json:"meta"`
}

type PartMeta struct {
	Partnum  int   `json:"part_num"`
	Filesize int64 `json:"file_size"`
}

type ReedsolomonStreamEncoder struct {
	encoder      reedsolomon.Encoder
	datashards   int
	parityshards int
}

type downloadResult struct {
	contentHash string
	reader      io.ReadCloser
	partNum     int
}

const CHUNK_SIZE = 64 * 1024

func (enc *ReedsolomonStreamEncoder) DownloadAndDecode(filePath string, blobberList []Blobber, writer io.Writer) error {
	metaArray := make([]MetaInfo, 0)
	actualSize := int64(0)
	for i := range blobberList {
		meta, err := enc.getPartMeta(filePath, blobberList[i].MetaURL)
		if err != nil {
			fmt.Println(err)
			continue
		}
		for j := range meta {
			var partMeta PartMeta
			err = json.Unmarshal([]byte(meta[j].CustomMeta), &partMeta)
			if err != nil {
				fmt.Println(err)
				continue
			}
			actualSize = partMeta.Filesize
			meta[j].customMetaPartInfo = partMeta
			meta[j].blobber = blobberList[i]
		}
		metaArray = append(metaArray, meta...)
	}

	if len(metaArray) < enc.datashards {
		return errors.New("Not enough shards to reconstruct the data")
	}

	ch := make(chan *downloadResult, len(metaArray))
	var wg sync.WaitGroup
	wg.Add(len(metaArray))

	for i := range metaArray {
		go enc.downloadPart(filePath, metaArray[i].blobber.DownloadURL, metaArray[i], ch, &wg)
	}

	wg.Wait()

	chanSize := enc.datashards + enc.parityshards

	dataReaders := make([]*downloadResult, chanSize)
	for range metaArray {
		result := <-ch

		if result.reader != nil {
			defer result.reader.Close()
			dataReaders[result.partNum] = result
		} else {
			dataReaders[result.partNum] = nil
		}
	}

	perShard := (actualSize + int64(enc.datashards) - 1) / int64(enc.datashards)
	remaining := perShard
	totalRemaining := actualSize

	for remaining > 0 {
		shards := make([][]byte, chanSize)
		sizeDone := 0
		count := 0
		for i := range dataReaders {
			if dataReaders[i] != nil {
				chunkSize := int64(math.Min(float64(remaining), float64(CHUNK_SIZE)))
				var bbuf bytes.Buffer
				bufWriter := bufio.NewWriter(&bbuf)
				_, err := io.CopyN(bufWriter, dataReaders[i].reader, chunkSize)

				if err != io.EOF && err != nil {
					continue
				}
				bufWriter.Flush()

				shards[i] = bbuf.Bytes()
				sizeDone = len(shards[i])
				count++
			} else {
				shards[i] = nil
			}
		}

		// Verify the shards
		ok, err := enc.encoder.Verify(shards)
		if ok {
			fmt.Println("No reconstruction needed")
		} else {
			fmt.Println("Verification failed. Reconstructing data")
			err = enc.encoder.Reconstruct(shards)
			if err != nil {
				fmt.Println("Reconstruct failed -", err)
				return err
			}
			ok, err = enc.encoder.Verify(shards)
			if !ok {
				fmt.Println("Verification failed after reconstruction, data likely corrupted.")
				return err
			}
		}

		joinSize := int(math.Min(float64(totalRemaining), float64(enc.datashards*sizeDone)))
		//destBytes := make([]byte, count * sizeDone)
		var bytesBuf bytes.Buffer
		bufWriter := bufio.NewWriter(&bytesBuf)
		// We don't know the exact filesize.
		err = enc.encoder.Join(bufWriter, shards, joinSize)
		if err != nil {
			fmt.Println("join failed")
			return err
		}
		bufWriter.Flush()
		writer.Write(bytesBuf.Bytes())
		remaining = remaining - int64(sizeDone)
		totalRemaining = int64(math.Abs(float64(totalRemaining - int64(enc.datashards*sizeDone))))
	}

	return nil
}

func (enc *ReedsolomonStreamEncoder) downloadPart(filePath string, baseURL string, metaInfo MetaInfo, ch chan<- *downloadResult, wg *sync.WaitGroup) {
	defer wg.Done()
	u, _ := url.Parse(baseURL)
	q := u.Query()
	dirPath, filename := filepath.Split(filePath)
	q.Set("path", dirPath)
	q.Set("filename", filename)
	q.Set("part_hash", metaInfo.ContentHash)
	u.RawQuery = q.Encode()
	fmt.Println(u.String())

	response, err := http.Get(u.String())
	//var reader io.ReadCloser
	//reader = response.Body
	if err != nil || response.StatusCode < 200 || response.StatusCode > 299 || response.ContentLength != metaInfo.Size {
		ch <- &downloadResult{contentHash: metaInfo.ContentHash, reader: nil, partNum: metaInfo.customMetaPartInfo.Partnum}
	} else {
		ch <- &downloadResult{contentHash: metaInfo.ContentHash, reader: response.Body, partNum: metaInfo.customMetaPartInfo.Partnum}
	}

	return
}

func (enc *ReedsolomonStreamEncoder) getPartMeta(filePath string, baseURL string) ([]MetaInfo, error) {
	u, _ := url.Parse(baseURL)
	q := u.Query()
	dirPath, filename := filepath.Split(filePath)
	q.Set("path", dirPath)
	q.Set("filename", filename)
	u.RawQuery = q.Encode()
	fmt.Println(u.String())

	var fileMeta FileMeta
	response, err := http.Get(u.String())
	if err != nil {
		return nil, err
	} else {
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&fileMeta)
		if err != nil {
			return nil, err
		}
		fmt.Println(len(fileMeta.Meta))
	}
	return fileMeta.Meta, nil
}

func (enc *ReedsolomonStreamEncoder) uploadFile(filename string, url string, dataChannel chan []byte, stopChannel chan bool, size int64, meta []byte, wg *sync.WaitGroup) {
	defer wg.Done()
	bodyReader, bodyWriter := io.Pipe()
	multiWriter := multipart.NewWriter(bodyWriter)
	go func() {
		//fmt.Println("here");
		fileWriter, err := multiWriter.CreateFormFile("uploadFile", filename)
		if err != nil {
			bodyWriter.CloseWithError(err)
			return
		}

		perShard := (size + int64(enc.datashards) - 1) / int64(enc.datashards)
		remaining := perShard

		for remaining > 0 {
			dataBytes := <-dataChannel
			fileWriter.Write(dataBytes)
			remaining = remaining - int64(len(dataBytes))
		}

		// Create a form field writer for field label
		metaWriter, err := multiWriter.CreateFormField("custom_meta")
		if err != nil {
			bodyWriter.CloseWithError(err)
			return
		}
		metaWriter.Write(meta)

		bodyWriter.CloseWithError(multiWriter.Close())
	}()
	contentType := multiWriter.FormDataContentType()

	resp, err := http.Post(url, contentType, bodyReader)
	fmt.Println("url", url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return

	}
	fmt.Println("resp", string(resp_body))
	return
}

func (enc *ReedsolomonStreamEncoder) Init(dataShards int, parityShards int) error {
	encoder, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return err
	}

	enc.encoder = encoder
	enc.datashards = dataShards
	enc.parityshards = parityShards

	return nil
}

func (enc *ReedsolomonStreamEncoder) readFile(inputfile io.Reader, dataChannels []chan []byte, stopChannel chan bool, size int64) {

	// Calculate number of bytes per shard.
	perShard := (size + int64(enc.datashards) - 1) / int64(enc.datashards)

	// Pad data to r.Shards*perShard.
	padding := make([]byte, (int64(enc.datashards)*perShard)-size)
	dataReader := io.MultiReader(inputfile, bytes.NewBuffer(padding))

	chunksPerShard := (perShard + int64(CHUNK_SIZE) - 1) / CHUNK_SIZE

	for ctr := int64(0); ctr < chunksPerShard; ctr++ {

		remaining := int64(math.Min(float64(perShard-(ctr*CHUNK_SIZE)), CHUNK_SIZE))
		//fmt.Println(remaining)

		b1 := make([]byte, remaining*int64(enc.datashards))

		_, err := dataReader.Read(b1)

		if err != nil {
			fmt.Println("reading failed")
			stopChannel <- true
			return
		}

		data, err := enc.encoder.Split(b1)
		if err != nil {
			fmt.Println("split failed", err)
			stopChannel <- true
			return
		}
		err = enc.encoder.Encode(data)
		if err != nil {
			fmt.Println("encode failed", err)
			stopChannel <- true
			return
		}
		for i := range data {
			dataChannels[i] <- data[i]
		}
	}
	return
}

func (enc *ReedsolomonStreamEncoder) EncodeAndUpload(filePath string, blobberList []Blobber) error {
	inputfile, err := os.Open(filePath)
	if err != nil {
		return err
	}

	inputstat, err := inputfile.Stat()
	if err != nil {
		return err
	}

	chanSize := enc.datashards + enc.parityshards

	dataChannels := make([]chan []byte, chanSize)
	var stopChannel chan bool
	for i := range dataChannels {
		dataChannels[i] = make(chan []byte)
	}
	size := inputstat.Size()
	go enc.readFile(inputfile, dataChannels, stopChannel, size)

	var wg sync.WaitGroup
	wg.Add(chanSize)

	filename := filepath.Base(filePath)

	for i := 0; i < chanSize; i++ {
		var meta PartMeta
		meta.Partnum = i
		meta.Filesize = size

		metaBytes, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		blobberindex := i % len(blobberList)
		go enc.uploadFile(filename, blobberList[blobberindex].UploadURL, dataChannels[i], stopChannel, size, metaBytes, &wg)
	}

	wg.Wait()
	return nil
}
