package storage

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"

	"0chain.net/core/common"
	"0chain.net/core/encryption"
	"0chain.net/core/node"
	"0chain.net/core/util"
	"0chain.net/validatorcore/storage/writemarker"

	"github.com/mitchellh/mapstructure"

	. "0chain.net/core/logging"
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

type Attributes struct {
	WhoPaysForReads common.WhoPays `json:"who_pays_for_reads,omitempty" mapstructure:"who_pays_for_reads"`
}

func (a *Attributes) String() string {
	if a == nil || (*a) == (Attributes{}) {
		return "{}"
	}
	var b, err = json.Marshal(a)
	if err != nil {
		return "{}"
	}
	return string(b)
}

type DirMetaData struct {
	CreationDate common.Timestamp `json:"creation_date" mapstructure:"creation_date"`
	Type         string           `json:"type" mapstructure:"type"`
	Name         string           `json:"name" mapstructure:"name"`
	Path         string           `json:"path" mapstructure:"path"`
	Hash         string           `json:"hash" mapstructure:"hash"`
	PathHash     string           `json:"path_hash" mapstructure:"path_hash"`
	NumBlocks    int64            `json:"num_of_blocks" mapstructure:"num_of_blocks"`
	AllocationID string           `json:"allocation_id"`
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

	return encryption.Hash(strings.Join(childHashes, ":"))
}

func (r *DirMetaData) GetNumBlocks() int64 {
	return r.NumBlocks
}
func (r *DirMetaData) GetType() string {
	return r.Type
}

type FileMetaData struct {
	DirMetaData    `mapstructure:",squash"`
	CustomMeta     string     `json:"custom_meta" mapstructure:"custom_meta"`
	ContentHash    string     `json:"content_hash" mapstructure:"content_hash"`
	Size           int64      `json:"size" mapstructure:"size"`
	MerkleRoot     string     `json:"merkle_root" mapstructure:"merkle_root"`
	ActualFileSize int64      `json:"actual_file_size" mapstructure:"actual_file_size"`
	ActualFileHash string     `json:"actual_file_hash" mapstructure:"actual_file_hash"`
	Attributes     Attributes `json:"attributes" mapstructure:"attributes" `
}

func (fr *FileMetaData) GetHashData() string {
	hashArray := make([]string, 0)
	hashArray = append(hashArray, fr.AllocationID)
	hashArray = append(hashArray, fr.Type)
	hashArray = append(hashArray, fr.Name)
	hashArray = append(hashArray, fr.Path)
	hashArray = append(hashArray, strconv.FormatInt(fr.Size, 10))
	hashArray = append(hashArray, fr.ContentHash)
	hashArray = append(hashArray, fr.MerkleRoot)
	hashArray = append(hashArray, strconv.FormatInt(fr.ActualFileSize, 10))
	hashArray = append(hashArray, fr.ActualFileHash)
	hashArray = append(hashArray, fr.Attributes.String())
	return strings.Join(hashArray, ":")
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
						Logger.Error("Hash mismatch for file.", zap.Any("hashdata", fileObj.GetHashData()), zap.Any("newhash", newHash), zap.Any("given_hash", fileObj.GetHash()))
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
							Logger.Error("Hash mismatch for directory.", zap.Any("newhash", newHash), zap.Any("given_hash", dirObj.GetHash()), zap.Any("dirObj", dirObj))
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
		return nil, common.NewError("hash_mismatch", "Object path error since there is a mismatch in the dir hashes. "+rootDir.Path)
	}
	return &rootDir, nil
}

func (op *ObjectPath) VerifyBlockNum(challengeRand int64) error {
	if op.RootObject.NumBlocks == 0 {
		Logger.Info("Challenge is on a empty allocation")
		return nil
	}
	r := rand.New(rand.NewSource(challengeRand))
	//rand.Seed(challengeRand)
	blockNum := r.Int63n(op.RootObject.NumBlocks)
	blockNum = blockNum + 1

	if op.RootObject.NumBlocks < blockNum {
		return common.NewError("invalid_block_num", "Invalid block number"+fmt.Sprint(op.RootObject.NumBlocks)+" / "+fmt.Sprint(blockNum))
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
				remainingBlocks = remainingBlocks - child.GetNumBlocks()
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
		Logger.Error("File for Block num was not found in object path", zap.Any("object_path", op), zap.Any("rand_seed", challengeRand), zap.Any("blocknum", blockNum), zap.Any("root_blocks", op.RootObject.NumBlocks))
		return common.NewError("invalid_object_path", "File for Block num was not found in object path")
	}

	if op.Meta.GetHash() != curRef.GetHash() {
		Logger.Error("Block num was not for the same file as object path", zap.Any("curRef", curRef), zap.Any("object_path", op), zap.Any("rand_seed", challengeRand), zap.Any("blocknum", blockNum), zap.Any("root_blocks", op.RootObject.NumBlocks))
		return common.NewError("invalid_object_path", "Block num was not for the same file as object path")
	}

	return nil
}

func (op *ObjectPath) VerifyPath(allocationID string) error {
	rootDir, err := op.Parse(op.Path, allocationID)
	op.RootObject = rootDir

	if err != nil {
		Logger.Error("Error parsing the object path", zap.Any("object_path", op))
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

type ChallengeRequest struct {
	ChallengeID  string                           `json:"challenge_id"`
	ObjPath      *ObjectPath                      `json:"object_path,omitempty"`
	WriteMarkers []*writemarker.WriteMarkerEntity `json:"write_markers,omitempty"`
	DataBlock    []byte                           `json:"data,omitempty"`
	MerklePath   *util.MTPath                     `json:"merkle_path,omitempty"`
}

func (cr *ChallengeRequest) VerifyChallenge(challengeObj *Challenge, allocationObj *Allocation) error {
	Logger.Info("Verifying object path", zap.Any("challenge_id", challengeObj.ID), zap.Any("seed", challengeObj.RandomNumber))
	err := cr.ObjPath.Verify(challengeObj.AllocationID, challengeObj.RandomNumber)
	if err != nil {
		return common.NewError("challenge_validation_failed", "Failed to verify the object path."+err.Error())
	}

	if cr.WriteMarkers == nil || len(cr.WriteMarkers) == 0 {
		return common.NewError("challenge_validation_failed", "Invalid write marker")
	}

	Logger.Info("Verifying write marker", zap.Any("challenge_id", challengeObj.ID))
	err = cr.WriteMarkers[0].WM.Verify(allocationObj.ID, challengeObj.AllocationRoot, cr.WriteMarkers[0].ClientPublicKey)
	if err != nil {
		return err
	}
	for i := 1; i < len(cr.WriteMarkers); i++ {
		err = cr.WriteMarkers[i].WM.Verify(allocationObj.ID, cr.WriteMarkers[i].WM.AllocationRoot, cr.WriteMarkers[i].ClientPublicKey)
		if err != nil {
			return err
		}
		if cr.WriteMarkers[i].WM.PreviousAllocationRoot != cr.WriteMarkers[i-1].WM.AllocationRoot {
			return common.NewError("write_marker_validation_failed", "Write markers chain is invalid")
		}
	}
	latestWM := cr.WriteMarkers[len(cr.WriteMarkers)-1].WM
	rootRef := cr.ObjPath.RootObject
	allocationRootCalculated := encryption.Hash(rootRef.Hash + ":" + strconv.FormatInt(int64(latestWM.Timestamp), 10))

	if latestWM.AllocationRoot != allocationRootCalculated {
		return common.NewError("challenge_validation_failed", "Allocation root does not match")
	}

	if rootRef.NumBlocks == 0 {
		return nil
	}

	Logger.Info("Verifying data block and merkle path", zap.Any("challenge_id", challengeObj.ID))
	contentHash := encryption.Hash(cr.DataBlock)
	merkleVerify := util.VerifyMerklePath(contentHash, cr.MerklePath, cr.ObjPath.Meta.MerkleRoot)
	if !merkleVerify {
		return common.NewError("challenge_validation_failed", "Failed to verify the merkle path for the data block")
	}
	return nil
}

type Challenge struct {
	ID             string         `json:"id"`
	Validators     []*StorageNode `json:"validators"`
	RandomNumber   int64          `json:"seed"`
	AllocationID   string         `json:"allocation_id"`
	Blobber        *StorageNode   `json:"blobber"`
	AllocationRoot string         `json:"allocation_root"`
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
