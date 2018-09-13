package encoder

import (
	"os"
	"sync"
	"path/filepath"
	"mime/multipart"
	"io/ioutil"
	"net/http"
	"fmt"
	"io"
	"encoding/json"
	"bytes"
	"math"
	"net/url"
	"errors"
	"github.com/klauspost/reedsolomon"
)

type MetaInfo struct {
	Filename string `json:"filename"`
	CustomMeta string `json:"custom_meta"`
	Size int64 `json:"size"`
	ContentHash string `json:"content_hash"`
	customMetaPartInfo PartMeta
}

type FileMeta struct {
	ID string `json:"id"`
	Meta []MetaInfo `json:"meta"`
}

type PartMeta struct {
	Partnum int `json:"part_num"`
	Filesize int64 `json:"file_size"`
}

type ReedsolomonStreamEncoder struct {
	encoder reedsolomon.StreamEncoder
	datashards int
	parityshards int
	wg sync.WaitGroup
}

type downloadResult struct {
	contentHash string
	reader io.ReadCloser
	partNum int
}

const CHUNK_SIZE = 64 * 1024

func (enc *ReedsolomonStreamEncoder) DownloadAndDecode(filePath string, blobberList []Blobber, writer io.Writer) (error) {
	metaArray := make([]MetaInfo, 0)
	actualSize := int64(0)
	for i:= range blobberList {
		meta, err := enc.getPartMeta(filePath, blobberList[i].MetaURL)
		if(err!=nil) {
			fmt.Println(err)
			continue
		}
		for j:= range meta {
        	var partMeta PartMeta 
        	err = json.Unmarshal([]byte(meta[j].CustomMeta), &partMeta)
        	if(err != nil) {
        		fmt.Println(err)
        		continue
        	}
        	actualSize = partMeta.Filesize
        	meta[j].customMetaPartInfo = partMeta
        }
        metaArray = append(metaArray, meta...)
	}

	if(len(metaArray) < enc.datashards) {
		return errors.New("Not enough shards to reconstruct the data")
	}

	ch := make(chan *downloadResult, len(metaArray))
	//var wg sync.WaitGroup
	//wg.Add(len(metaArray))

	for i:= range metaArray {
		go enc.downloadPart(filePath, blobberList[i%len(blobberList)].DownloadURL, metaArray[i], ch)
	}
	
	shards := make([]io.Reader, enc.datashards+enc.parityshards)
	//inputReader := make([]io.Reader, enc.datashards+enc.parityshards)
	for range metaArray {
		//go enc.createFile("big.txt", ch, &wg)
		result := <- ch
		
		//pr, pw := io.Pipe()
		//tr := io.TeeReader(result.reader, pw)
		if(result.reader != nil) {
			shards[result.partNum] = result.reader
		} else {
			shards[result.partNum] = nil
		}
		
		//inputReader[result.partNum] = pr 

		defer result.reader.Close()
	}
	//wg.Wait()

	// Verify the shards
	// ok, err := enc.encoder.Verify(shards)

	// if ok {
	// 	fmt.Println("No reconstruction needed")
	// } else {
	// 	fmt.Println("Verification required")
	// 	return err
	// }
	// fmt.Println(len(shards))
	// We don't know the exact filesize.
	err := enc.Join(writer, shards, actualSize)

	return err
}

func (enc *ReedsolomonStreamEncoder) createFile(filename string, ch <-chan *downloadResult, wg *sync.WaitGroup) {
	defer wg.Done()
	result := <- ch
	destfilename := fmt.Sprintf("%s.%d", "big.txt", result.partNum)
	fmt.Println("file to be created" , destfilename)
	f, _ := os.Create(destfilename)
 	
	//checkErr(err)
	// copy from reader data into writer file
	io.Copy(f,result.reader)
	
	fmt.Println("file created" , destfilename)
	f.Close()
	//result.reader.Close()
}

func (enc *ReedsolomonStreamEncoder) downloadPart(filePath string, baseURL string, metaInfo MetaInfo, ch chan<- *downloadResult) {
	u, _ := url.Parse(baseURL)
	q := u.Query()
	dirPath, filename := filepath.Split(filePath)
	q.Set("path", dirPath)
	q.Set("filename", filename)
	q.Set("part_hash", metaInfo.ContentHash)
	u.RawQuery = q.Encode()
	fmt.Println(u.String())
	
	response, err := http.Get(u.String())
    if err != nil || response.StatusCode < 200 || response.StatusCode > 299 {
        ch <- &downloadResult{contentHash: metaInfo.ContentHash, reader: nil, partNum : metaInfo.customMetaPartInfo.Partnum}
    } else {
        ch <- &downloadResult{contentHash: metaInfo.ContentHash, reader: response.Body, partNum : metaInfo.customMetaPartInfo.Partnum}
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
        if(err != nil) {
        	return nil, err
        }
        fmt.Println(len(fileMeta.Meta))
    }
    return fileMeta.Meta, nil
}


func(enc *ReedsolomonStreamEncoder) uploadFile(filename string, url string, reader io.Reader, wg *sync.WaitGroup, meta []byte) error {
	defer wg.Done()
	bodyReader, bodyWriter := io.Pipe()
	multiWriter := multipart.NewWriter(bodyWriter)
	go func() {
		fileWriter, err := multiWriter.CreateFormFile("uploadFile", filename)
		if err != nil {
			bodyWriter.CloseWithError(err)
			return
		}

		//iocopy
		_, err = io.Copy(fileWriter, reader)
		if err != nil {
			bodyWriter.CloseWithError(err)
			return
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
		return err
	}
	defer resp.Body.Close()
	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err

	}
	fmt.Println("resp", string(resp_body))
	return nil
}


func (enc *ReedsolomonStreamEncoder) Split(data io.Reader, dst []io.Writer, size int64) error {
	if size == 0 {
		return reedsolomon.ErrShortData
	}
	if len(dst) != enc.datashards {
		return reedsolomon.ErrInvShardNum
	}

	for i := range dst {
		if dst[i] == nil {
			return reedsolomon.StreamWriteError{Err: reedsolomon.ErrShardNoData, Stream: i}
		}
	}

	// Calculate number of bytes per shard.
	perShard := (size + int64(enc.datashards) - 1) / int64(enc.datashards)

	// Pad data to r.Shards*perShard.
	padding := make([]byte, (int64(enc.datashards)*perShard)-size)
	data = io.MultiReader(data, bytes.NewBuffer(padding))

	chunksPerShard := (perShard + int64(CHUNK_SIZE) - 1) / CHUNK_SIZE
	ctr := int64(0)
	for ctr < chunksPerShard {
		remaining := int64(math.Min(float64(perShard - (ctr * CHUNK_SIZE)), CHUNK_SIZE))
		// Split into equal-length shards and copy.
		for i := range dst {
			_, err := io.CopyN(dst[i], data, remaining)
			if err != io.EOF && err != nil {
				return err
			}
		}
		ctr++
	}

	return nil
}

func (enc *ReedsolomonStreamEncoder) Join(dst io.Writer, shards []io.Reader, outSize int64) error {
	// Do we have enough shards?
	if len(shards) < enc.datashards {
		fmt.Println("Too few shards");
		return reedsolomon.ErrTooFewShards
	}

	// Trim off parity shards if any
	shards = shards[:enc.datashards]
	for i := range shards {
		if shards[i] == nil {
			return reedsolomon.StreamReadError{Err: reedsolomon.ErrShardNoData, Stream: i}
		}
	}
	// Calculate number of bytes per shard.
	perShard := (outSize + int64(enc.datashards) - 1) / int64(enc.datashards)

	// Pad data to r.Shards*perShard.
	padding := (int64(enc.datashards)*perShard)-outSize
	chunksPerShard := (perShard + int64(CHUNK_SIZE) - 1) / CHUNK_SIZE
	ctr := int64(0)
	for ctr < chunksPerShard {
		remaining := int64(math.Min(float64(perShard - (ctr * CHUNK_SIZE)), CHUNK_SIZE))
		// Split into equal-length shards and copy.
		for i := range shards {
			if(ctr + 1 == chunksPerShard && i + 1 == len(shards)){
				remaining = remaining - padding
			}
			_, err := io.CopyN(dst, shards[i], remaining)
			if err != io.EOF && err != nil {
				return err
			}
		}
		ctr++
	}
	return nil
}

func(enc *ReedsolomonStreamEncoder) Init (dataShards int, parityShards int) (error) {
	encoder, err := reedsolomon.NewStreamC(dataShards, parityShards, true, true)
	if(err != nil) {
		return err
	}

	enc.encoder = encoder
	enc.datashards = dataShards
	enc.parityshards = parityShards

	return nil
}

func (enc *ReedsolomonStreamEncoder) EncodeAndUpload (filePath string, blobberList []Blobber) (error){
	inputfile, err := os.Open(filePath)
	if(err !=nil) {
		return err
	}

	inputstat, err := inputfile.Stat()
	if(err !=nil) {
		return err
	}
	wg := enc.wg
	wg.Add(enc.datashards + enc.parityshards + 2)

	fileout := make([]io.Writer, enc.datashards)
	httpout := make([]io.Writer, enc.datashards)
	allout := make([]io.Writer, enc.datashards)

	filein := make([]io.Reader, enc.datashards)
	filename := filepath.Base(filePath)
	for i := range allout {
		var meta PartMeta
		meta.Partnum = i
		meta.Filesize = inputstat.Size()

		metaBytes,err := json.Marshal(meta)
		if(err!=nil) {
			return err
		}


		pr, pw := io.Pipe()
		npr, npw := io.Pipe()

		fileout[i] = pw
		httpout[i] = npw
		allout[i] = io.MultiWriter(pw, npw)
		filein[i] = pr

		if(err!=nil) {
			return err
		}
		blobberindex := i % len(blobberList)
		
		go enc.uploadFile(filename,blobberList[blobberindex].UploadURL, npr, &wg, metaBytes)
	}

	// Create parity output writers
	parity := make([]io.Writer, enc.parityshards)
	for i := range parity {
		pr, pw := io.Pipe()
		parity[i] = pw
		var meta PartMeta
		meta.Partnum = enc.datashards+i
		meta.Filesize = inputstat.Size()

		metaBytes,err := json.Marshal(meta)
		if(err!=nil) {
			return err
		}
		blobberindex := (enc.datashards+i) % len(blobberList)
		go enc.uploadFile(filename,blobberList[blobberindex].UploadURL, pr, &wg, metaBytes)

	}

	go func() {
		defer wg.Done()
		// Encode parity
		err = enc.encoder.Encode(filein, parity)
		if(err!=nil) {
			return
		}
		for i:= range parity {
			parity[i].(*io.PipeWriter).Close()
		}
	}()

	go func() {
		defer wg.Done()
		// Do the split
		err = enc.Split(inputfile, allout, inputstat.Size())
		if(err!=nil) {
			return
		}
		for i := range allout {
			httpout[i].(*io.PipeWriter).Close()
			fileout[i].(*io.PipeWriter).Close()
		}
	}()

	wg.Wait()
	return nil
}