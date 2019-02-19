package allocation

import (
	"context"
	"fmt"

	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
)

type TokenStatus int

const (
	NEW       TokenStatus = 0
	COMMITTED TokenStatus = 1
)

type DeleteToken struct {
	FilePathHash string           `json:"file_path_hash"`
	FileRefHash  string           `json:"file_ref_hash"`
	AllocationID string           `json:"allocation_id"`
	Size         int64            `json:"size"`
	BlobberID    string           `json:"blobber_id"`
	Timestamp    common.Timestamp `json:"timestamp"`
	ClientID     string           `json:"client_id"`
	Signature    string           `json:"signature"`
	Status       TokenStatus      `json:"status"`
}

func (dt *DeleteToken) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", dt.FileRefHash, dt.FilePathHash, dt.AllocationID, dt.BlobberID, dt.ClientID, dt.Size, dt.Timestamp)
	return hashData
}

func (dt *DeleteToken) VerifySignature(clientPubKey string) bool {
	if len(clientPubKey) == 0 {
		return false
	}
	hashData := dt.GetHashData()
	signatureHash := encryption.Hash(hashData)
	ok, err := encryption.Verify(clientPubKey, dt.Signature, signatureHash)
	if err != nil {
		return false
	}
	return ok
}

var deleteTokenMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func DeleteTokenProvider() datastore.Entity {
	t := &DeleteToken{}
	return t
}

func SetupDeleteTokenEntity(store datastore.Store) {
	deleteTokenMetaData = datastore.MetadataProvider()
	deleteTokenMetaData.Name = "deletetoken"
	deleteTokenMetaData.DB = "deletetoken"
	deleteTokenMetaData.Provider = DeleteTokenProvider
	deleteTokenMetaData.Store = store

	datastore.RegisterEntityMetadata("deletetoken", deleteTokenMetaData)
}

func (dt *DeleteToken) GetEntityMetadata() datastore.EntityMetadata {
	return deleteTokenMetaData
}
func (dt *DeleteToken) SetKey(key datastore.Key) {
	//wm.ID = datastore.ToString(key)
}

func (dt *DeleteToken) GetKey() string {
	return dt.GetEntityMetadata().GetDBName() + ":" + dt.FileRefHash
}

func (dt *DeleteToken) Read(ctx context.Context, key datastore.Key) error {
	return deleteTokenMetaData.GetStore().Read(ctx, key, dt)
}
func (dt *DeleteToken) Write(ctx context.Context) error {
	return deleteTokenMetaData.GetStore().Write(ctx, dt)
}
func (dt *DeleteToken) Delete(ctx context.Context) error {
	return nil
}
