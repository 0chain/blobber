package handler

import (
	"bytes"
	"errors"

	zencryption "github.com/0chain/gosdk/zboxcore/encryption"
)

// ChunkEncoder encode/decode chunk data
type ChunkEncoder interface {
	// Encode encode chunk data if it is necessary
	Encode(chunkSize int, data []byte) ([]byte, error)
}

// RawChunkEncoder raw chunk data
type RawChunkEncoder struct {
}

// Encode read chunk data
func (r *RawChunkEncoder) Encode(chunkSize int, data []byte) ([]byte, error) {
	return data, nil
}

// PREChunkEncoder encode and decode chunk data with PREEncryptionScheme if file is shared with auth token
type PREChunkEncoder struct {
	EncryptedKey              string
	ReEncryptionKey           string
	ClientEncryptionPublicKey string
}

// Encode encode chunk data with PREEncryptionScheme for subscriber to download it
func (r *PREChunkEncoder) Encode(chunkSize int, data []byte) ([]byte, error) {
	encscheme := zencryption.NewEncryptionScheme()
	if _, err := encscheme.Initialize(""); err != nil {
		return nil, err
	}

	if err := encscheme.InitForDecryption("filetype:audio", r.EncryptedKey); err != nil {
		return nil, err
	}

	totalSize := len(data)
	result := []byte{}

	for i := 0; i < totalSize; i += chunkSize {
		encMsg := new(zencryption.EncryptedMessage)

		nextIndex := i + chunkSize
		var chunkData []byte
		if nextIndex > totalSize {
			chunkData = make([]byte, totalSize-i)
			nextIndex = totalSize
		} else {
			chunkData = make([]byte, chunkSize)
		}

		copy(chunkData, data[i:nextIndex])

		encMsg.EncryptedData = chunkData[EncryptionHeaderSize:]

		headerBytes := chunkData[:EncryptionHeaderSize]
		headerBytes = bytes.Trim(headerBytes, "\x00")

		if len(headerBytes) != EncryptionHeaderSize {
			return nil, errors.New("Block has invalid encryption header")
		}

		encMsg.MessageChecksum, encMsg.OverallChecksum = string(headerBytes[:128]), string(headerBytes[128:])
		encMsg.EncryptedKey = encscheme.GetEncryptedKey()

		reEncMsg, err := encscheme.ReEncrypt(encMsg, r.ReEncryptionKey, r.ClientEncryptionPublicKey)
		if err != nil {
			return nil, err
		}

		encData, err := reEncMsg.Marshal()
		if err != nil {
			return nil, err
		}
		// 256 bytes to save ReEncryption header instead of 2048 EncryptionHeader
		result = append(result, encData...)
	}
	return result, nil
}
