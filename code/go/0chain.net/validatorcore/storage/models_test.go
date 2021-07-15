package storage_test

import (
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/validatorcore/storage"

	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestDirMetaData_GetNumBlocks(t *testing.T) {
	t.Skip("covered in TestObjectPath_VerifyBlockNum")
}

func TestDirMetaData_GetHash(t *testing.T) {
	t.Skip("covered in TestObjectPath_Parse")
}

func TestDirMetaData_CalculateHash(t *testing.T) {
	tests := []struct {
		name string
		dmd  storage.DirMetaData
		want string
	}{
		{
			name: "without children",
			dmd: storage.DirMetaData{
				Hash: "hash0",
			},
			want: "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
		},
		{
			name: "with children",
			dmd: storage.DirMetaData{
				Hash: "hash0",
				Children: []storage.ObjectEntity{
					&storage.DirMetaData{
						Hash: "hash1",
					},
				},
			},
			want: "5f2ec2d6c64c3e0bed5af21673e7e824d1f91f484ebcfe7a4758949e6d9eb6e0",
		},
		{
			name: "with nested children",
			dmd: storage.DirMetaData{
				Hash: "hash0",
				Children: []storage.ObjectEntity{
					&storage.DirMetaData{
						Hash: "hash1",
						Children: []storage.ObjectEntity{
							&storage.DirMetaData{
								Hash: "hash2",
							},
						},
					},
				},
			},
			want: "5f2ec2d6c64c3e0bed5af21673e7e824d1f91f484ebcfe7a4758949e6d9eb6e0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dmd.CalculateHash()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDirMetaData_GetType(t *testing.T) {
	t.Skip("covered in TestObjectPath_VerifyBlockNum")
}

func TestFileMetaData_GetHashData(t *testing.T) {
	tests := []struct {
		name string
		fmd  storage.FileMetaData
		want string
	}{
		{
			name: "with Attributes.WhoPays = WhoPaysOwner",
			fmd: storage.FileMetaData{
				DirMetaData: storage.DirMetaData{},
			},
			want: "::::0:::0::{}",
			// want: "::::0:::0::{\"who_pays_for_reads\":0}",
		},
		{
			name: "with Attributes.WhoPays = WhoPays3rdParty",
			fmd: storage.FileMetaData{
				DirMetaData: storage.DirMetaData{},
			},
			want: "::::0:::0::{\"who_pays_for_reads\":1}",
		},
		{
			name: "with Attributes.WhoPays = nil",
			fmd: storage.FileMetaData{
				DirMetaData: storage.DirMetaData{},
			},
			want: "::::0:::0::{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fmd.GetHashData()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFileMetaData_NumBlocks(t *testing.T) {
	t.Skip("covered in TestObjectPath_VerifyBlockNum")
}

func TestFileMetaData_GetHash(t *testing.T) {
	t.Skip("covered in TestObjectPath_Parse")
}

func TestFileMetaData_CalculateHash(t *testing.T) {
	tests := []struct {
		name string
		fmd  storage.FileMetaData
		want string
	}{
		{
			name: "with Attributes.WhoPays = WhoPaysOwner",
			fmd:  storage.FileMetaData{},
			want: "f78718c8ad33d8b97fe902dabc36df401f82c88bde608ab85005d332ac24de43",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fmd.CalculateHash()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFileMetaData_GetType(t *testing.T) {
	t.Skip("covered in TestObjectPath_VerifyBlockNum")
}

func TestObjectPath_Parse(t *testing.T) {
	logging.Logger = zap.New(nil) // FIXME to avoid complains

	tests := []struct {
		name       string
		objPath    *storage.ObjectPath
		input      map[string]interface{}
		allocID    string
		want       *storage.DirMetaData
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "dir/file path: hash mismatch",
			objPath: &storage.ObjectPath{},
			input: map[string]interface{}{
				"path": "dir1",
				"hash": "b25a7f67d4206d77fca08a48a06eba893c59077ea61435f71b31d098ea2f7991",
				"type": "d",
				"list": []map[string]interface{}{
					map[string]interface{}{
						"path": "file.txt",
						"hash": "b25a7f67d4206d77fca08a48a06eba893c59077ea61435f71b31d098ea2f7991",
						"type": "f",
					},
				},
			},
			allocID:    "1",
			wantErr:    true,
			wantErrMsg: "Object path error since there is a mismatch in the file hashes.",
		},
		{
			name:    "dir/dir/file path: hash mismatch",
			objPath: &storage.ObjectPath{},
			input: map[string]interface{}{
				"path": "dir1",
				"hash": "a02b02080606e78e165fe5a42f8b0087ff82617a1f9c26cc95e269fd653c5a72",
				"type": "d",
				"list": []map[string]interface{}{
					map[string]interface{}{
						"path": "dir2",
						"hash": "a02b02080606e78e165fe5a42f8b0087ff82617a1f9c26cc95e269fd653c5a72",
						"type": "d",
						"list": []map[string]interface{}{
							map[string]interface{}{
								"path": "file.txt",
								"hash": "87177591985fdf5c010d7781f0dc82b5d3c40b6bf8892b3c69000eb000f1e33a",
								"type": "f",
							},
						},
					},
				},
			},
			allocID:    "1",
			wantErr:    true,
			wantErrMsg: "Object path error since there is a mismatch in the dir hashes.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.objPath.Parse(tt.input, tt.allocID)
			if !tt.wantErr {
				require.NoError(t, err)
			} else {
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestObjectPath_VerifyBlockNum(t *testing.T) {
	logging.Logger = zap.New(nil) // FIXME to avoid complains
	tests := []struct {
		name       string
		objPath    *storage.ObjectPath
		rand       int64
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "0",
			objPath: &storage.ObjectPath{
				RootObject: &storage.DirMetaData{
					NumBlocks: int64(0),
				},
			},
			rand: 1,
		},
		{
			name: "not found",
			objPath: &storage.ObjectPath{
				RootObject: &storage.DirMetaData{
					NumBlocks: int64(1),
				},
			},
			rand:       1,
			wantErr:    true,
			wantErrMsg: "File for Block num was not found in object path",
		},
		{
			name: "not found with children",
			objPath: &storage.ObjectPath{
				RootObject: &storage.DirMetaData{
					NumBlocks: int64(1),
					Children: []storage.ObjectEntity{
						&storage.DirMetaData{
							Type:      storage.DIRECTORY,
							NumBlocks: int64(1),
						},
					},
				},
			},
			rand:       1,
			wantErr:    true,
			wantErrMsg: "File for Block num was not found in object path",
		},
		{
			name: "found wrong hash",
			objPath: &storage.ObjectPath{
				RootObject: &storage.DirMetaData{
					NumBlocks: int64(1),
					Children: []storage.ObjectEntity{
						&storage.FileMetaData{
							DirMetaData: storage.DirMetaData{
								Type:      storage.FILE,
								NumBlocks: int64(1),
							},
						},
					},
				},
				Meta: &storage.FileMetaData{
					DirMetaData: storage.DirMetaData{
						Hash: "hash",
					},
				},
			},
			rand:       1,
			wantErr:    true,
			wantErrMsg: "Block num was not for the same file as object path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.objPath.VerifyBlockNum(tt.rand)
			if !tt.wantErr {
				require.NoError(t, err)
			} else {
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestObjectPath_VerifyPath(t *testing.T) {
	logging.Logger = zap.New(nil) // FIXME to avoid complains

	tests := []struct {
		name       string
		objPath    *storage.ObjectPath
		allocID    string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "invalid input",
			objPath:    &storage.ObjectPath{},
			allocID:    "1",
			wantErr:    true,
			wantErrMsg: "Object path error since there is a mismatch in the dir hashes.",
		},
		{
			name: "empty",
			objPath: &storage.ObjectPath{
				RootHash: "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
				Path: map[string]interface{}{
					"hash": "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
				},
				RootObject: &storage.DirMetaData{
					CreationDate: common.Timestamp(0),
					Type:         "",
					Name:         "",
					Path:         "",
					Hash:         "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
					PathHash:     "",
					NumBlocks:    int64(0),
					AllocationID: "",
					Children:     nil,
				},
			},
			allocID: "1",
		},
		{
			name: "root",
			objPath: &storage.ObjectPath{
				RootHash: "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
				Path: map[string]interface{}{
					"path": "file.txt",
					"hash": "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
					"type": "f",
				},
				RootObject: &storage.DirMetaData{
					CreationDate: common.Timestamp(0),
					Type:         storage.FILE,
					Name:         "",
					Path:         "file.txt",
					Hash:         "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
					PathHash:     "",
					NumBlocks:    int64(0),
					AllocationID: "",
					Children:     nil,
				},
			},
			allocID: "1",
		},
		{
			name: "dir/file",
			objPath: &storage.ObjectPath{
				RootHash: "b25a7f67d4206d77fca08a48a06eba893c59077ea61435f71b31d098ea2f7991",
				Path: map[string]interface{}{
					"path": "dir1",
					"hash": "b25a7f67d4206d77fca08a48a06eba893c59077ea61435f71b31d098ea2f7991",
					"type": "d",
					"list": []map[string]interface{}{
						map[string]interface{}{
							"path": "file.txt",
							"hash": "87177591985fdf5c010d7781f0dc82b5d3c40b6bf8892b3c69000eb000f1e33a",
							"type": "f",
						},
					},
				},
				RootObject: &storage.DirMetaData{
					CreationDate: common.Timestamp(0),
					Type:         storage.DIRECTORY,
					Name:         "",
					Path:         "dir1",
					Hash:         "b25a7f67d4206d77fca08a48a06eba893c59077ea61435f71b31d098ea2f7991",
					PathHash:     "",
					NumBlocks:    int64(0),
					AllocationID: "",
					Children: []storage.ObjectEntity{
						&storage.FileMetaData{
							DirMetaData: storage.DirMetaData{
								CreationDate: common.Timestamp(0),
								Type:         storage.FILE,
								Name:         "",
								Path:         "file.txt",
								Hash:         "87177591985fdf5c010d7781f0dc82b5d3c40b6bf8892b3c69000eb000f1e33a",
								PathHash:     "",
								NumBlocks:    int64(0),
								AllocationID: "1",
								Children:     nil,
							},
							CustomMeta:     "",
							ContentHash:    "",
							Size:           int64(0),
							MerkleRoot:     "",
							ActualFileSize: int64(0),
							ActualFileHash: "",
						},
					},
				},
			},
			allocID: "1",
		},
		{
			name: "dir/file path: hash mismatch",
			objPath: &storage.ObjectPath{
				RootHash: "87177591985fdf5c010d7781f0dc82b5d3c40b6bf8892b3c69000eb000f1e33a",
				Path: map[string]interface{}{
					"path": "dir1",
					"hash": "b25a7f67d4206d77fca08a48a06eba893c59077ea61435f71b31d098ea2f7991",
					"type": "d",
					"list": []map[string]interface{}{
						map[string]interface{}{
							"path": "file.txt",
							"hash": "87177591985fdf5c010d7781f0dc82b5d3c40b6bf8892b3c69000eb000f1e33a",
							"type": "f",
						},
					},
				},
				RootObject: &storage.DirMetaData{
					CreationDate: common.Timestamp(0),
					Type:         storage.DIRECTORY,
					Name:         "",
					Path:         "dir1",
					Hash:         "b25a7f67d4206d77fca08a48a06eba893c59077ea61435f71b31d098ea2f7991",
					PathHash:     "",
					NumBlocks:    int64(0),
					AllocationID: "",
					Children: []storage.ObjectEntity{
						&storage.FileMetaData{
							DirMetaData: storage.DirMetaData{
								CreationDate: common.Timestamp(0),
								Type:         storage.FILE,
								Name:         "",
								Path:         "file.txt",
								Hash:         "87177591985fdf5c010d7781f0dc82b5d3c40b6bf8892b3c69000eb000f1e33a",
								PathHash:     "",
								NumBlocks:    int64(0),
								AllocationID: "1",
								Children:     nil,
							},
							CustomMeta:     "",
							ContentHash:    "",
							Size:           int64(0),
							MerkleRoot:     "",
							ActualFileSize: int64(0),
							ActualFileHash: "",
						},
					},
				},
			},
			allocID:    "1",
			wantErr:    true,
			wantErrMsg: " Root Hash does not match with object path",
		},
		{
			name: "dir/dir/file",
			objPath: &storage.ObjectPath{
				RootHash: "a02b02080606e78e165fe5a42f8b0087ff82617a1f9c26cc95e269fd653c5a72",
				Path: map[string]interface{}{
					"path": "dir1",
					"hash": "a02b02080606e78e165fe5a42f8b0087ff82617a1f9c26cc95e269fd653c5a72",
					"type": "d",
					"list": []map[string]interface{}{
						map[string]interface{}{
							"path": "dir2",
							"hash": "b25a7f67d4206d77fca08a48a06eba893c59077ea61435f71b31d098ea2f7991",
							"type": "d",
							"list": []map[string]interface{}{
								map[string]interface{}{
									"path": "file.txt",
									"hash": "87177591985fdf5c010d7781f0dc82b5d3c40b6bf8892b3c69000eb000f1e33a",
									"type": "f",
								},
							},
						},
					},
				},
				RootObject: &storage.DirMetaData{
					CreationDate: common.Timestamp(0),
					Type:         storage.DIRECTORY,
					Name:         "",
					Path:         "dir1",
					Hash:         "a02b02080606e78e165fe5a42f8b0087ff82617a1f9c26cc95e269fd653c5a72",
					PathHash:     "",
					NumBlocks:    int64(0),
					AllocationID: "",
					Children: []storage.ObjectEntity{
						&storage.DirMetaData{
							CreationDate: common.Timestamp(0),
							Type:         storage.DIRECTORY,
							Name:         "",
							Path:         "dir2",
							Hash:         "b25a7f67d4206d77fca08a48a06eba893c59077ea61435f71b31d098ea2f7991",
							PathHash:     "",
							NumBlocks:    int64(0),
							AllocationID: "1",
							Children: []storage.ObjectEntity{
								&storage.FileMetaData{
									DirMetaData: storage.DirMetaData{
										CreationDate: common.Timestamp(0),
										Type:         storage.FILE,
										Name:         "",
										Path:         "file.txt",
										Hash:         "87177591985fdf5c010d7781f0dc82b5d3c40b6bf8892b3c69000eb000f1e33a",
										PathHash:     "",
										NumBlocks:    int64(0),
										AllocationID: "1",
										Children:     nil,
									},
									CustomMeta:     "",
									ContentHash:    "",
									Size:           int64(0),
									MerkleRoot:     "",
									ActualFileSize: int64(0),
									ActualFileHash: "",
								},
							},
						},
					},
				},
			},
			allocID: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.objPath.VerifyPath(tt.allocID)
			if !tt.wantErr {
				require.NoError(t, err)
			} else {
				t.Log(err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestObjectPath_Verify(t *testing.T) {
	t.Skip("covered")
}

func TestChallengeRequest_VerifyChallenge(t *testing.T) {
	logging.Logger = zap.New(nil) // FIXME to avoid complains

	tests := []struct {
		name       string
		chReq      *storage.ChallengeRequest
		ch         *storage.Challenge
		alloc      *storage.Allocation
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "verify object path fails",
			chReq: &storage.ChallengeRequest{
				ObjPath: &storage.ObjectPath{
					RootHash: "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
					Path: map[string]interface{}{
						"path": "file.txt",
						"hash": "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
						"type": "f",
					},
					RootObject: &storage.DirMetaData{
						CreationDate: common.Timestamp(0),
						Type:         storage.FILE,
						Name:         "",
						Path:         "file.txt",
						Hash:         "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
						PathHash:     "",
						NumBlocks:    int64(0),
						AllocationID: "",
						Children:     nil,
					},
				},
			},
			ch: &storage.Challenge{
				RandomNumber: int64(1),
				AllocationID: "2",
			},
			wantErr:    true,
			wantErrMsg: "Invalid write marker",
		},
		{
			name: "invalid write marker",
			chReq: &storage.ChallengeRequest{
				ObjPath: &storage.ObjectPath{
					RootHash: "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
					Path: map[string]interface{}{
						"path": "file.txt",
						"hash": "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
						"type": "f",
					},
					RootObject: &storage.DirMetaData{
						CreationDate: common.Timestamp(0),
						Type:         storage.FILE,
						Name:         "",
						Path:         "file.txt",
						Hash:         "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a",
						PathHash:     "",
						NumBlocks:    int64(0),
						AllocationID: "",
						Children:     nil,
					},
				},
			},
			ch: &storage.Challenge{
				RandomNumber: int64(1),
				AllocationID: "1",
			},
			wantErr:    true,
			wantErrMsg: "Invalid write marker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.chReq.VerifyChallenge(tt.ch, tt.alloc)
			if !tt.wantErr {
				require.NoError(t, err)
			} else {
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestValidationTicket_Sign(t *testing.T) {
	err := setupModelsTest(t)
	require.NoError(t, err)

	vt := storage.ValidationTicket{
		ChallengeID:  "challenge_id",
		BlobberID:    "blobber_id",
		ValidatorID:  "validator_id",
		ValidatorKey: "validator_key",
		Result:       true,
		Timestamp:    common.Now(),
	}

	err = vt.Sign()
	require.NoError(t, err)
}

func setupModelsTest(t *testing.T) error {
	t.Helper()
	config.Configuration = config.Config{
		SignatureScheme: "bls0chain",
	}
	sigSch := zcncrypto.NewSignatureScheme("bls0chain")
	wallet, err := sigSch.GenerateKeys()
	if err != nil {
		return err
	}

	node.Self.SetKeys(wallet.Keys[0].PublicKey, wallet.Keys[0].PrivateKey)
	return nil
}
