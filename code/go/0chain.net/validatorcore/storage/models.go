package storage

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"reflect"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/validatorcore/storage/writemarker"
	"github.com/0chain/gosdk/core/util"

	"github.com/mitchellh/mapstructure"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

const (
	FILE      = "f"
	DIRECTORY = "d"
)

const LIST_TAG = "list"
const TYPE_TAG = "type"

type ObjectEntity interface {
	GetNumBlocks() int64
	GetHash() string
	CalculateHash() string
	GetType() string
}

type DirMetaData struct {
	CreationDate common.Timestamp `json:"created_at" mapstructure:"created_at"`
	FileID       string           `json:"file_id" mapstructure:"file_id"`
	Type         string           `json:"type" mapstructure:"type"`
	Name         string           `json:"name" mapstructure:"name"`
	Path         string           `json:"path" mapstructure:"path"`
	Hash         string           `json:"hash" mapstructure:"hash"`
	PathHash     string           `json:"path_hash" mapstructure:"path_hash"`
	NumBlocks    int64            `json:"num_of_blocks" mapstructure:"num_of_blocks"`
	AllocationID string           `json:"allocation_id" mapstructure:"allocation_id"`
	Children     []ObjectEntity   `json:"-"`
}

func (r *DirMetaData) GetHash() string {
	return r.Hash
}

func (r *DirMetaData) CalculateHash() string {
	childHashes := make([]string, len(r.Children))
	for index, childRef := range r.Children {
		childHashes[index] = childRef.GetHash()
	}

	return encryption.Hash(r.GetHashData() + strings.Join(childHashes, ":"))
}

func (r *DirMetaData) GetNumBlocks() int64 {
	return r.NumBlocks
}

func (r *DirMetaData) GetType() string {
	return r.Type
}

func (r *DirMetaData) GetHashData() string {
	return fmt.Sprintf("%s:%s:%s", r.AllocationID, r.Path, r.FileID)

}

type FileMetaData struct {
	DirMetaData     `mapstructure:",squash"`
	CustomMeta      string `json:"custom_meta" mapstructure:"custom_meta"`
	ValidationRoot  string `json:"validation_root" mapstructure:"validation_root"`
	Size            int64  `json:"size" mapstructure:"size"`
	FixedMerkleRoot string `json:"fixed_merkle_root" mapstructure:"fixed_merkle_root"`
	ActualFileSize  int64  `json:"actual_file_size" mapstructure:"actual_file_size"`
	ActualFileHash  string `json:"actual_file_hash" mapstructure:"actual_file_hash"`
	ChunkSize       int64  `json:"chunk_size" mapstructure:"chunk_size"`
}

func (fr *FileMetaData) GetHashData() string {
	return fmt.Sprintf(
		"%s:%s:%s:%s:%d:%s:%s:%d:%s:%d:%s",
		fr.AllocationID,
		fr.Type, // don't need to add it as well
		fr.Name, // don't see any utility as fr.Path below has name in it
		fr.Path,
		fr.Size,
		fr.ValidationRoot,
		fr.FixedMerkleRoot,
		fr.ActualFileSize,
		fr.ActualFileHash,
		fr.ChunkSize,
		fr.FileID,
	)
}

func (fr *FileMetaData) GetHash() string {
	return fr.Hash
}

func (fr *FileMetaData) CalculateHash() string {
	return encryption.Hash(fr.GetHashData())
}

func (fr *FileMetaData) GetNumBlocks() int64 {
	return fr.NumBlocks
}
func (fr *FileMetaData) GetType() string {
	return fr.Type
}

type ObjectPath struct {
	RootHash     string                 `json:"root_hash"`
	Meta         *FileMetaData          `json:"meta_data"`
	Path         map[string]interface{} `json:"path"`
	FileBlockNum int64                  `json:"file_block_num"`
	RootObject   *DirMetaData           `json:"-"`
}

func (op *ObjectPath) Parse(input map[string]interface{}, allocationID string) (*DirMetaData, error) {
	var rootDir DirMetaData
	err := mapstructure.Decode(input, &rootDir)
	if err != nil {
		return nil, err
	}
	t, ok := input[LIST_TAG]
	if ok {
		switch reflect.TypeOf(t).Kind() {
		case reflect.Slice:
			s := reflect.ValueOf(t)
			rootDir.Children = make([]ObjectEntity, s.Len())
			for i := 0; i < s.Len(); i++ {
				object := s.Index(i).Interface().(map[string]interface{})

				if object[TYPE_TAG] == FILE {
					var fileObj FileMetaData
					err := mapstructure.Decode(object, &fileObj)
					if err != nil {
						return nil, err
					}
					fileObj.AllocationID = allocationID
					newHash := fileObj.CalculateHash()
					if newHash != fileObj.GetHash() {
						logging.Logger.Error("Hash mismatch for file.", zap.String("hashdata", fileObj.GetHashData()), zap.String("newhash", newHash), zap.String("given_hash", fileObj.GetHash()))
						return nil, common.NewError("hash_mismatch", "Object path error since there is a mismatch in the file hashes. "+fileObj.Path)
					}
					rootDir.Children[i] = &fileObj
				} else {
					dirObj := &DirMetaData{}
					if _, ok := object[LIST_TAG]; ok {
						dirObj, err = op.Parse(object, allocationID)
						if err != nil {
							return nil, err
						}
						dirObj.AllocationID = allocationID
						newHash := dirObj.CalculateHash()
						if newHash != dirObj.GetHash() {
							logging.Logger.Error("Hash mismatch for directory.", zap.String("newhash", newHash), zap.String("given_hash", dirObj.GetHash()), zap.String("dirObj_path", dirObj.Path))
							return nil, common.NewError("hash_mismatch", "Object path error since there is a mismatch in the dir hashes. "+dirObj.Path)
						}
					} else {
						err = mapstructure.Decode(object, dirObj)
						if err != nil {
							return nil, err
						}
					}

					dirObj.AllocationID = allocationID
					rootDir.Children[i] = dirObj
				}
			}
		default:
			return nil, common.NewError("invalid_object_path", "Invalid object path. List should be an array")
		}
	}

	newHash := rootDir.CalculateHash()

	if newHash != rootDir.GetHash() {
		logging.Logger.Error("Hash mismatch for root directory", zap.String("newhash", newHash), zap.String("given_hash", rootDir.GetHash()))
		return nil, common.NewError("hash_mismatch", "Object path error since there is a mismatch in the dir hashes. "+rootDir.Path)
	}
	return &rootDir, nil
}

func (op *ObjectPath) VerifyBlockNum(challengeRand int64) error {
	if op.RootObject.NumBlocks == 0 {
		logging.Logger.Info("Challenge is on a empty allocation")
		return nil
	}
	r := rand.New(rand.NewSource(challengeRand))
	blockNum := r.Int63n(op.RootObject.NumBlocks)
	blockNum++

	if op.RootObject.NumBlocks < blockNum {
		return common.NewError("invalid_block_num", fmt.Sprintf("Invalid block number %d/%d", op.RootObject.NumBlocks, blockNum))
	}

	found := false
	var curRef ObjectEntity
	curRef = op.RootObject
	remainingBlocks := blockNum

	for !found {
		if len(curRef.(*DirMetaData).Children) == 0 {
			break
		}
		for _, child := range curRef.(*DirMetaData).Children {
			if child.GetNumBlocks() < remainingBlocks {
				remainingBlocks -= child.GetNumBlocks()
				continue
			}
			if child.GetType() == FILE {
				found = true
				curRef = child
				break
			}
			curRef = child
			break
		}
	}
	if !found {
		logging.Logger.Error("File for Block num was not found in object path", zap.Int64("rand_seed", challengeRand), zap.Int64("blocknum", blockNum), zap.Int64("root_blocks", op.RootObject.NumBlocks))
		return common.NewError("invalid_object_path", "File for Block num was not found in object path")
	}

	if op.Meta.GetHash() != curRef.GetHash() {
		logging.Logger.Error("Block num was not for the same file as object path", zap.String("curRef_hash", curRef.GetHash()), zap.String("object_path_hash", op.Meta.GetHash()), zap.Int64("rand_seed", challengeRand), zap.Int64("blocknum", blockNum), zap.Int64("root_blocks", op.RootObject.NumBlocks))
		return common.NewError("invalid_object_path", "Block num was not for the same file as object path")
	}

	return nil
}

func (op *ObjectPath) VerifyPath(allocationID string) error {
	rootDir, err := op.Parse(op.Path, allocationID)
	op.RootObject = rootDir

	if err != nil {
		logging.Logger.Error("Error parsing the object path")
		return common.NewError("invalid_object_path", "Error parsing the object path. "+err.Error())
	}
	if op.RootHash != rootDir.Hash {
		return common.NewError("invalid_object_path", "Root Hash does not match with object path")
	}
	return nil
}

func (op *ObjectPath) Verify(allocationID string, challengeRand int64) error {
	err := op.VerifyPath(allocationID)
	if err != nil {
		return err
	}
	err = op.VerifyBlockNum(challengeRand)
	return err
}

type Allocation struct {
	ID             string           `json:"id"`
	DataShards     int              `json:"data_shards"`
	ParityShards   int              `json:"parity_shards"`
	Size           int64            `json:"size"`
	UsedSize       int64            `json:"used_size"`
	Expiration     common.Timestamp `json:"expiration_date"`
	Owner          string           `json:"owner_id"`
	OwnerPublicKey string           `json:"owner_public_key"`
}

type ChallengeProof struct {
	Proof   [][]byte `json:"proof"`
	Data    []byte   `json:"data"`
	LeafInd int      `json:"leaf_ind"`
}

type ChallengeRequest struct {
	ChallengeID    string                           `json:"challenge_id"`
	ObjPath        *ObjectPath                      `json:"object_path,omitempty"`
	WriteMarkers   []*writemarker.WriteMarkerEntity `json:"write_markers,omitempty"`
	ChallengeProof *ChallengeProof                  `json:"challenge_proof"`
}

func (cr *ChallengeRequest) verifyBlockNum(challengeObj *Challenge) error {
	r := rand.New(rand.NewSource(challengeObj.RandomNumber))
	blockNum := r.Intn(util.FixedMerkleLeaves)
	if blockNum != cr.ChallengeProof.LeafInd {
		return fmt.Errorf("expected block num %d, got %d", blockNum, cr.ChallengeProof.LeafInd)
	}
	return nil
}

func (cr *ChallengeRequest) VerifyChallenge(challengeObj *Challenge, allocationObj *Allocation) error {
	logging.Logger.Info("Verifying object path", zap.String("challenge_id", challengeObj.ID), zap.Int64("seed", challengeObj.RandomNumber))
	if cr.ObjPath != nil {
		err := cr.ObjPath.Verify(challengeObj.AllocationID, challengeObj.RandomNumber)
		if err != nil {
			return common.NewError("challenge_validation_failed", "Failed to verify the object path."+err.Error())
		}
	}

	if len(cr.WriteMarkers) == 0 {
		return common.NewError("challenge_validation_failed", "Invalid write marker")
	}

	logging.Logger.Info("Verifying write marker", zap.String("challenge_id", challengeObj.ID))
	err := cr.WriteMarkers[0].WM.Verify(allocationObj.ID, challengeObj.AllocationRoot, cr.WriteMarkers[0].ClientPublicKey)
	if err != nil {
		return common.NewError("write_marker_validation_failed", "Failed to verify the write marker. "+err.Error())
	}
	if cr.WriteMarkers[0].WM.Timestamp != challengeObj.Timestamp {
		return common.NewError("write_marker_validation_failed", "Write marker timestamp does not match with challenge timestamp")
	}
	for i := 1; i < len(cr.WriteMarkers); i++ {
		err = cr.WriteMarkers[i].WM.Verify(allocationObj.ID, cr.WriteMarkers[i].WM.AllocationRoot, cr.WriteMarkers[i].ClientPublicKey)
		if err != nil {
			return err
		}
		if cr.WriteMarkers[i].WM.PreviousAllocationRoot != cr.WriteMarkers[i-1].WM.AllocationRoot {
			if cr.WriteMarkers[i].WM.Timestamp != cr.WriteMarkers[i-1].WM.Timestamp {
				return common.NewError("write_marker_validation_failed", "Write markers chain is invalid")
			}
		}
	}
	latestWM := cr.WriteMarkers[len(cr.WriteMarkers)-1].WM
	if cr.ObjPath != nil {
		rootRef := cr.ObjPath.RootObject
		allocationRootCalculated := rootRef.Hash

		if latestWM.AllocationRoot != allocationRootCalculated {
			return common.NewError("challenge_validation_failed", "Allocation root does not match")
		}

		if rootRef.NumBlocks == 0 {
			return nil
		}
	} else {
		if latestWM.AllocationRoot != "" || latestWM.PreviousAllocationRoot != "" {
			return common.NewError("challenge_validation_failed", "Allocation root is not empty")
		}
		return nil
	}

	if cr.ChallengeProof == nil {
		return common.NewError("challenge_validation_failed", "Challenge proof is missing")
	}

	err = cr.verifyBlockNum(challengeObj)
	if err != nil {
		return common.NewError("challenge_validation_failed", "Failed to verify block num."+err.Error())
	}

	logging.Logger.Info("Verifying data block and merkle path", zap.String("challenge_id", challengeObj.ID))
	fHash := encryption.ShaHash(cr.ChallengeProof.Data)
	fixedMerkleRoot, _ := hex.DecodeString(cr.ObjPath.Meta.FixedMerkleRoot)
	fmp := &util.FixedMerklePath{
		LeafHash: fHash,
		RootHash: fixedMerkleRoot,
		Nodes:    cr.ChallengeProof.Proof,
		LeafInd:  cr.ChallengeProof.LeafInd,
	}

	if !fmp.VerifyMerklePath() {
		logging.Logger.Error("Failed to verify merkle path for the data block")
		return common.NewError("challenge_validation_failed", "Failed to verify the merkle path for the data block")
	}
	return nil
}

type Challenge struct {
	ID             string           `json:"id"`
	Validators     []*StorageNode   `json:"validators"`
	RandomNumber   int64            `json:"seed"`
	AllocationID   string           `json:"allocation_id"`
	AllocationRoot string           `json:"allocation_root"`
	BlobberID      string           `json:"blobber_id"`
	Timestamp      common.Timestamp `json:"timestamp"`
}

type ValidationTicket struct {
	ChallengeID  string           `json:"challenge_id"`
	BlobberID    string           `json:"blobber_id"`
	ValidatorID  string           `json:"validator_id"`
	ValidatorKey string           `json:"validator_key"`
	Result       bool             `json:"success"`
	Message      string           `json:"message"`
	MessageCode  string           `json:"message_code"`
	Timestamp    common.Timestamp `json:"timestamp"`
	Signature    string           `json:"signature"`
}

func (vt *ValidationTicket) Sign() error {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v", vt.ChallengeID, vt.BlobberID, vt.ValidatorID, vt.ValidatorKey, vt.Result, vt.Timestamp)
	hash := encryption.Hash(hashData)
	signature, err := node.Self.Sign(hash)
	vt.Signature = signature
	return err
}
