package storage_test

import (
	"math/rand"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/validatorcore/storage"

	"github.com/0chain/gosdk/core/util"
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
			want: "763c38be0664691418d38f5ccde0162c9ff11fbda1b946d56476bdaa90fd13d6",
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
			want: "cf04ad708ac8dc61996ce6e3719cfb18ab7fe6cc0fa42f4f162bd1c8c1902c95",
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
			want: "cf04ad708ac8dc61996ce6e3719cfb18ab7fe6cc0fa42f4f162bd1c8c1902c95",
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

func TestFileMetaData_NumBlocks(t *testing.T) {
	t.Skip("covered in TestObjectPath_VerifyBlockNum")
}

func TestFileMetaData_GetHash(t *testing.T) {
	t.Skip("covered in TestObjectPath_Parse")
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
				"path":    "dir1",
				"hash":    "c2a5549b3c592cf5f2fd73a8abf650ef44438d22d6c5b25b28edb4016b7cebdc",
				"type":    "d",
				"file_id": "2",
				"list": []map[string]interface{}{
					{
						"path":    "dir2",
						"hash":    "99f8c50accd7090635d9f1fa7094724665a2aede0fb629139e1502fa1cae8954",
						"type":    "d",
						"file_id": "3",
						"list": []map[string]interface{}{
							{
								"path":    "file.txt",
								"hash":    "f33f3cb4a59eba8826b1e8174770732fd6ef1289a8d852ae32d9f192ce4b1041",
								"type":    "f",
								"file_id": "4",
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
				RootHash: "216dc0baa286411f6ddf295eed45b9193eabd8f1a4d7264945d15f7e430afdd1",
				Path: map[string]interface{}{
					"hash":          "216dc0baa286411f6ddf295eed45b9193eabd8f1a4d7264945d15f7e430afdd1",
					"allocation_id": "1",
				},
				RootObject: &storage.DirMetaData{
					CreationDate: common.Timestamp(0),
					Type:         "",
					Name:         "",
					Path:         "",
					Hash:         "216dc0baa286411f6ddf295eed45b9193eabd8f1a4d7264945d15f7e430afdd1",
					PathHash:     "",
					NumBlocks:    int64(0),
					AllocationID: "1",
					Children:     nil,
				},
			},
			allocID: "1",
		},
		{
			name: "root",
			objPath: &storage.ObjectPath{
				RootHash: "36d3966ff34eb8cff4b8b4395023317df55ecd7805e270cc437c447569f77436",
				Path: map[string]interface{}{
					"path":          "file.txt",
					"hash":          "36d3966ff34eb8cff4b8b4395023317df55ecd7805e270cc437c447569f77436",
					"type":          "f",
					"allocation_id": "1",
				},
				RootObject: &storage.DirMetaData{
					CreationDate: common.Timestamp(0),
					Type:         storage.FILE,
					Name:         "",
					Path:         "file.txt",
					Hash:         "36d3966ff34eb8cff4b8b4395023317df55ecd7805e270cc437c447569f77436",
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
				RootHash: "b60c880b3933746f478668aedcd8aeed759cd7cf16236b2ed7deac42919978fe",
				Path: map[string]interface{}{
					"path":          "dir1",
					"hash":          "b60c880b3933746f478668aedcd8aeed759cd7cf16236b2ed7deac42919978fe",
					"type":          "d",
					"file_id":       "2",
					"allocation_id": "1",
					"list": []map[string]interface{}{
						{
							"path":    "file.txt",
							"hash":    "12994107b906551b07350276e9b775af0c4689b1b70e656e75fae5c82aa9d823",
							"type":    "f",
							"file_id": "3",
						},
					},
				},
				RootObject: &storage.DirMetaData{
					CreationDate: common.Timestamp(0),
					Type:         storage.DIRECTORY,
					Name:         "",
					Path:         "dir1",
					FileID:       "2",
					Hash:         "b60c880b3933746f478668aedcd8aeed759cd7cf16236b2ed7deac42919978fe",
					PathHash:     "",
					NumBlocks:    int64(0),
					AllocationID: "1",
					Children: []storage.ObjectEntity{
						&storage.FileMetaData{
							DirMetaData: storage.DirMetaData{
								CreationDate: common.Timestamp(0),
								Type:         storage.FILE,
								Name:         "",
								FileID:       "3",
								Path:         "file.txt",
								Hash:         "12994107b906551b07350276e9b775af0c4689b1b70e656e75fae5c82aa9d823",
								PathHash:     "",
								NumBlocks:    int64(0),
								AllocationID: "1",
								Children:     nil,
							},
							CustomMeta:      "",
							ValidationRoot:  "",
							Size:            int64(0),
							FixedMerkleRoot: "",
							ActualFileSize:  int64(0),
							ActualFileHash:  "",
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
					"path":          "dir1",
					"hash":          "b60c880b3933746f478668aedcd8aeed759cd7cf16236b2ed7deac42919978fe",
					"type":          "d",
					"file_id":       "2",
					"allocation_id": "1",
					"list": []map[string]interface{}{
						{
							"path":    "file.txt",
							"hash":    "12994107b906551b07350276e9b775af0c4689b1b70e656e75fae5c82aa9d823",
							"type":    "f",
							"file_id": "3",
						},
					},
				},
				RootObject: &storage.DirMetaData{
					CreationDate: common.Timestamp(0),
					Type:         storage.DIRECTORY,
					Name:         "",
					Path:         "dir1",
					FileID:       "2",
					Hash:         "8b1e23f3ca7b660354ae147fe259664cd2b7661f2b254ed09f47d925801d1be6",
					PathHash:     "",
					NumBlocks:    int64(0),
					AllocationID: "1",
					Children: []storage.ObjectEntity{
						&storage.FileMetaData{
							DirMetaData: storage.DirMetaData{
								CreationDate: common.Timestamp(0),
								Type:         storage.FILE,
								Name:         "",
								FileID:       "3",
								Path:         "file.txt",
								Hash:         "12994107b906551b07350276e9b775af0c4689b1b70e656e75fae5c82aa9d823",
								PathHash:     "",
								NumBlocks:    int64(0),
								AllocationID: "1",
								Children:     nil,
							},
							CustomMeta:      "",
							ValidationRoot:  "",
							Size:            int64(0),
							FixedMerkleRoot: "",
							ActualFileSize:  int64(0),
							ActualFileHash:  "",
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
				RootHash: "628e52776eeb0d198e74cd90f576d785531898da12c6f49e8ab5d40a01143bfe",
				Path: map[string]interface{}{
					"path":          "dir1",
					"hash":          "628e52776eeb0d198e74cd90f576d785531898da12c6f49e8ab5d40a01143bfe",
					"type":          "d",
					"file_id":       "2",
					"allocation_id": "1",
					"list": []map[string]interface{}{
						{
							"path":          "dir2",
							"hash":          "fe7864ff75fb0eeb4c8a630443914219861515f2cc7f5c3e95b43a83590b8ec5",
							"type":          "d",
							"file_id":       "3",
							"allocation_id": "1",
							"list": []map[string]interface{}{
								{
									"path":    "file.txt",
									"hash":    "f33f3cb4a59eba8826b1e8174770732fd6ef1289a8d852ae32d9f192ce4b1041",
									"type":    "f",
									"file_id": "4",
								},
							},
						},
					},
				},
				RootObject: &storage.DirMetaData{
					CreationDate: 0,
					FileID:       "2",
					Type:         storage.DIRECTORY,
					Name:         "",
					Path:         "dir1",
					Hash:         "628e52776eeb0d198e74cd90f576d785531898da12c6f49e8ab5d40a01143bfe",
					PathHash:     "",
					NumBlocks:    int64(0),
					AllocationID: "1",
					Children: []storage.ObjectEntity{
						&storage.DirMetaData{
							CreationDate: 0,
							Type:         storage.DIRECTORY,
							FileID:       "3",
							Name:         "",
							Path:         "dir2",
							Hash:         "fe7864ff75fb0eeb4c8a630443914219861515f2cc7f5c3e95b43a83590b8ec5",
							PathHash:     "",
							NumBlocks:    int64(0),
							AllocationID: "1",
							Children: []storage.ObjectEntity{
								&storage.FileMetaData{
									DirMetaData: storage.DirMetaData{
										CreationDate: 0,
										FileID:       "4",
										Type:         storage.FILE,
										Name:         "",
										Path:         "file.txt",
										Hash:         "f33f3cb4a59eba8826b1e8174770732fd6ef1289a8d852ae32d9f192ce4b1041",
										PathHash:     "",
										NumBlocks:    int64(0),
										AllocationID: "1",
										Children:     nil,
									},
									CustomMeta:      "",
									ValidationRoot:  "",
									Size:            int64(0),
									FixedMerkleRoot: "",
									ActualFileSize:  int64(0),
									ActualFileHash:  "",
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
				require.Contains(t, err.Error(), tt.wantErrMsg)
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
					RootHash: "198ce68546759504a976a524f8b7ab7102ca027ecf3018cad4a5e38f7c3b2889",
					Path: map[string]interface{}{
						"path": "file.txt",
						"hash": "198ce68546759504a976a524f8b7ab7102ca027ecf3018cad4a5e38f7c3b2889",
						"type": "f",
					},
					RootObject: &storage.DirMetaData{
						CreationDate: common.Timestamp(0),
						Type:         storage.FILE,
						Name:         "",
						Path:         "file.txt",
						Hash:         "198ce68546759504a976a524f8b7ab7102ca027ecf3018cad4a5e38f7c3b2889",
						PathHash:     "",
						NumBlocks:    int64(0),
						AllocationID: "",
						Children:     nil,
					},
				},
				ChallengeProof: func() *storage.ChallengeProof {
					randomNumber := int64(1)
					r := rand.New(rand.NewSource(randomNumber))
					ind := r.Intn(util.FixedMerkleLeaves)

					return &storage.ChallengeProof{
						LeafInd: ind,
					}
				}(),
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
					RootHash: "198ce68546759504a976a524f8b7ab7102ca027ecf3018cad4a5e38f7c3b2889",
					Path: map[string]interface{}{
						"path": "file.txt",
						"hash": "198ce68546759504a976a524f8b7ab7102ca027ecf3018cad4a5e38f7c3b2889",
						"type": "f",
					},
					RootObject: &storage.DirMetaData{
						CreationDate: common.Timestamp(0),
						Type:         storage.FILE,
						Name:         "",
						Path:         "file.txt",
						Hash:         "198ce68546759504a976a524f8b7ab7102ca027ecf3018cad4a5e38f7c3b2889",
						PathHash:     "",
						NumBlocks:    int64(0),
						AllocationID: "",
						Children:     nil,
					},
				},
				ChallengeProof: func() *storage.ChallengeProof {
					randomNumber := int64(1)
					r := rand.New(rand.NewSource(randomNumber))
					ind := r.Intn(util.FixedMerkleLeaves)

					return &storage.ChallengeProof{
						LeafInd: ind,
					}
				}(),
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
