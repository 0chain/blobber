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
			want: "f21ce1bdeefa1a99e9c611a4ae207efdffe5008e057486d7df93b91c78022440",
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
			want: "9dd3eebc02b5b973440113dcdc2c3f81cc6ed888aecc890ebc86c6b163f4487e",
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
			want: "9dd3eebc02b5b973440113dcdc2c3f81cc6ed888aecc890ebc86c6b163f4487e",
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
				RootHash: "9ed470d11d340098abf99015a6b52fbda55f18f470733544c777ec3d127b169a",
				Path: map[string]interface{}{
					"hash":          "9ed470d11d340098abf99015a6b52fbda55f18f470733544c777ec3d127b169a",
					"allocation_id": "1",
					"file_id":       "1",
				},
				RootObject: &storage.DirMetaData{
					CreationDate: common.Timestamp(0),
					Type:         "",
					Name:         "",
					Path:         "",
					Hash:         "9ed470d11d340098abf99015a6b52fbda55f18f470733544c777ec3d127b169a",
					PathHash:     "",
					NumBlocks:    int64(0),
					AllocationID: "1",
					Children:     nil,
					FileID:       "1",
				},
			},
			allocID: "1",
		},
		{
			name: "root",
			objPath: &storage.ObjectPath{
				RootHash: "980b103f2d78856eb8c5be014bbf7f12bb074eb61ac9bcfbb46ad6e4486cc737",
				Path: map[string]interface{}{
					"path":          "file.txt",
					"hash":          "980b103f2d78856eb8c5be014bbf7f12bb074eb61ac9bcfbb46ad6e4486cc737",
					"type":          "f",
					"allocation_id": "1",
				},
				RootObject: &storage.DirMetaData{
					CreationDate: common.Timestamp(0),
					Type:         storage.FILE,
					Name:         "",
					Path:         "file.txt",
					Hash:         "980b103f2d78856eb8c5be014bbf7f12bb074eb61ac9bcfbb46ad6e4486cc737",
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
				RootHash: "7dc9b7734a27401b9d55d28e45f88738d99596345679375c321ba6cb9c449820",
				Path: map[string]interface{}{
					"path":          "dir1",
					"hash":          "7dc9b7734a27401b9d55d28e45f88738d99596345679375c321ba6cb9c449820",
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
					Hash:         "7dc9b7734a27401b9d55d28e45f88738d99596345679375c321ba6cb9c449820",
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
					"hash":          "7dc9b7734a27401b9d55d28e45f88738d99596345679375c321ba6cb9c449820",
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
					Hash:         "7dc9b7734a27401b9d55d28e45f88738d99596345679375c321ba6cb9c449820",
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
				RootHash: "a2c5d9cb929ea365d28a7d024e2c9bdd2d9e55a4d456235b1fcecfb7efb072f9",
				Path: map[string]interface{}{
					"path":          "dir1",
					"hash":          "a2c5d9cb929ea365d28a7d024e2c9bdd2d9e55a4d456235b1fcecfb7efb072f9",
					"type":          "d",
					"file_id":       "2",
					"allocation_id": "1",
					"list": []map[string]interface{}{
						{
							"path":          "dir2",
							"hash":          "632d0271b78791b24970321b823b9987b23e812f31bbed6c0229e7dc0e0c037a",
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
					Hash:         "a2c5d9cb929ea365d28a7d024e2c9bdd2d9e55a4d456235b1fcecfb7efb072f9",
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
							Hash:         "632d0271b78791b24970321b823b9987b23e812f31bbed6c0229e7dc0e0c037a",
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
					RootHash: "980b103f2d78856eb8c5be014bbf7f12bb074eb61ac9bcfbb46ad6e4486cc737",
					Path: map[string]interface{}{
						"path":          "file.txt",
						"hash":          "980b103f2d78856eb8c5be014bbf7f12bb074eb61ac9bcfbb46ad6e4486cc737",
						"type":          "f",
						"allocation_id": "1",
					},
					RootObject: &storage.DirMetaData{
						CreationDate: common.Timestamp(0),
						Type:         storage.FILE,
						Name:         "",
						Path:         "file.txt",
						Hash:         "980b103f2d78856eb8c5be014bbf7f12bb074eb61ac9bcfbb46ad6e4486cc737",
						PathHash:     "",
						NumBlocks:    int64(0),
						AllocationID: "1",
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
					RootHash: "980b103f2d78856eb8c5be014bbf7f12bb074eb61ac9bcfbb46ad6e4486cc737",
					Path: map[string]interface{}{
						"path":          "file.txt",
						"hash":          "980b103f2d78856eb8c5be014bbf7f12bb074eb61ac9bcfbb46ad6e4486cc737",
						"type":          "f",
						"allocation_id": "1",
					},
					RootObject: &storage.DirMetaData{
						CreationDate: common.Timestamp(0),
						Type:         storage.FILE,
						Name:         "",
						Path:         "file.txt",
						Hash:         "980b103f2d78856eb8c5be014bbf7f12bb074eb61ac9bcfbb46ad6e4486cc737",
						PathHash:     "",
						NumBlocks:    int64(0),
						AllocationID: "1",
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

	err = vt.Sign(0)
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
