package handler

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/automigration"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/stretchr/testify/require"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/spf13/viper"

	"testing"

	"gorm.io/gorm"

	"github.com/0chain/gosdk/core/zcncrypto"
)

func randString(n int) string {
	const hexLetters = "abcdef0123456789"

	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteByte(hexLetters[rand.Intn(len(hexLetters))])
	}
	return sb.String()
}

func setupGrpcTests(t *testing.T) (blobbergrpc.BlobberServiceClient, *TestDataController) {
	startGRPCServer(t)

	bClient, _, err := makeTestClient()
	require.NoError(t, err)

	setupIntegrationTestConfig(t)

	db, err := datastore.UseInMemory()
	require.NoError(t, err)

	// Enable to see SQL logging
	//db.Logger = db.Logger.LogMode(logger.Info)

	err = automigration.DropSchemas(db)
	require.NoError(t, err)

	err = automigration.MigrateSchema(db)
	require.NoError(t, err)

	// Recreate timestamp columns to be sqlite compatible.
	// Columns with `timestamp without time zone` cannot be parsed properly in sqlite.
	db.Exec("ALTER TABLE `reference_objects` DROP COLUMN `created_at`")
	db.Exec("ALTER TABLE `reference_objects` ADD COLUMN `created_at` timestamp NOT NULL DEFAULT current_timestamp")
	db.Exec("DROP INDEX `idx_updated_at`")
	db.Exec("ALTER TABLE `reference_objects` DROP COLUMN `updated_at`")
	db.Exec("ALTER TABLE `reference_objects` ADD COLUMN `updated_at` timestamp NOT NULL DEFAULT current_timestamp")
	db.Exec("CREATE INDEX `idx_updated_at` ON `reference_objects`(`updated_at`)")
	db.Exec("ALTER TABLE `challenges` DROP COLUMN `created_at`")
	db.Exec("ALTER TABLE `challenges` ADD COLUMN `created_at` timestamp NOT NULL DEFAULT current_timestamp")
	db.Exec("ALTER TABLE `challenges` DROP COLUMN `updated_at`")
	db.Exec("ALTER TABLE `challenges` ADD COLUMN `updated_at` timestamp NOT NULL DEFAULT current_timestamp")
	db.Exec("ALTER TABLE `collaborators` DROP COLUMN `created_at`")
	db.Exec("ALTER TABLE `collaborators` ADD COLUMN `created_at` timestamp NOT NULL DEFAULT current_timestamp")
	db.Exec("ALTER TABLE `commit_meta_txns` DROP COLUMN `created_at`")
	db.Exec("ALTER TABLE `commit_meta_txns` ADD COLUMN `created_at` timestamp NOT NULL DEFAULT current_timestamp")
	db.Exec("ALTER TABLE `marketplace_share_info` DROP COLUMN `available_at`")
	db.Exec("ALTER TABLE `marketplace_share_info` ADD COLUMN `available_at` timestamp NOT NULL DEFAULT current_timestamp")

	tdController := NewTestDataController(db)

	return bClient, tdController
}

type TestDataController struct {
	db *gorm.DB
}

func NewTestDataController(db *gorm.DB) *TestDataController {
	return &TestDataController{db: db}
}

func (c *TestDataController) AddGetAllocationTestData() error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, repairer_id, is_immutable)
VALUES ('exampleId' ,'exampleTransaction','exampleOwnerId','exampleOwnerPublicKey',` + fmt.Sprint(expTime) + `, 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *TestDataController) AddGetFileMetaDataTestData(allocationTx, pubKey string) error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, repairer_id, is_immutable)
VALUES ('exampleId' ,'` + allocationTx + `','exampleOwnerId','` + pubKey + `',` + fmt.Sprint(expTime) + `, 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO reference_objects (id, allocation_id, path_hash,lookup_hash,type,name,path,hash,custom_meta,content_hash,merkle_root,actual_file_hash,mimetype,write_marker,thumbnail_hash, actual_thumbnail_hash)
VALUES (1234,'exampleId','exampleId:examplePath','exampleId:examplePath','f','filename','examplePath','someHash','customMeta','contentHash','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash');
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO commit_meta_txns (ref_id,txn_id)
VALUES (1234,'someTxn');
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO collaborators (ref_id, client_id)
VALUES (1234, 'someClient');
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *TestDataController) AddGetFileStatsTestData(allocationTx, pubKey string) error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, repairer_id, is_immutable)
VALUES ('exampleId' ,'` + allocationTx + `','exampleOwnerId','` + pubKey + `',` + fmt.Sprint(expTime) + `, 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO reference_objects (id, allocation_id, path_hash,lookup_hash,type,name,path,hash,custom_meta,content_hash,merkle_root,actual_file_hash,mimetype,write_marker,thumbnail_hash, actual_thumbnail_hash)
VALUES (1234,'exampleId','exampleId:examplePath','exampleId:examplePath','f','filename','examplePath','someHash','customMeta','contentHash','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash');
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *TestDataController) AddListEntitiesTestData(allocationTx, pubkey string) error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, repairer_id, is_immutable)
VALUES ('exampleId' ,'` + allocationTx + `','exampleOwnerId','` + pubkey + `',` + fmt.Sprint(expTime) + `, 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(` 
INSERT INTO reference_objects (id, level,  allocation_id, path_hash,lookup_hash,type,name,path,parent_path,hash,custom_meta,content_hash,merkle_root,actual_file_hash,mimetype,write_marker,thumbnail_hash, actual_thumbnail_hash)
VALUES (1233, 1, 'exampleId','exampleId:root','exampleId:root','d','/','/','','roothash','rootmeta','roothash','rootmerkleRoot','actualRootHash','mimetype','writeMarker','thumbnailHash','actualRootThumbnailHash');
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(` 
INSERT INTO reference_objects (id, level, allocation_id, path_hash,lookup_hash,type,name,path,parent_path,hash,custom_meta,content_hash,merkle_root,actual_file_hash,mimetype,write_marker,thumbnail_hash, actual_thumbnail_hash)
VALUES (1234, 1, 'exampleId','exampleId:exampleDir','exampleId:exampleDir','d','filename','/exampleDir','/','someHash','customMeta','contentHash','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash');
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO reference_objects (id, level, allocation_id, path_hash,lookup_hash,type,name,path,parent_path,hash,custom_meta,content_hash,merkle_root,actual_file_hash,mimetype,write_marker,thumbnail_hash, actual_thumbnail_hash)
VALUES (1235, 2, 'exampleId','exampleId:examplePath','exampleId:examplePath','f','filename','/exampleDir/examplePath','/exampleDir','someHash','customMeta','contentHash','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash');
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *TestDataController) AddGetObjectPathTestData(allocationTx, pubKey string) error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, repairer_id, is_immutable)
VALUES ('exampleId' ,'` + allocationTx + `','exampleOwnerId','` + pubKey + `',` + fmt.Sprint(expTime) + `, 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *TestDataController) AddGetReferencePathTestData(allocationTx, pubkey string) error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, repairer_id, is_immutable)
VALUES ('exampleId' ,'` + allocationTx + `','exampleOwnerId','` + pubkey + `',` + fmt.Sprint(expTime) + `, 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *TestDataController) AddGetObjectTreeTestData(allocationTx, pubkey string) error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, repairer_id, is_immutable)
VALUES ('exampleId' ,'` + allocationTx + `','exampleOwnerId','` + pubkey + `',` + fmt.Sprint(expTime) + `, 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO reference_objects (id, allocation_id, path_hash,lookup_hash,type,name,path,hash,custom_meta,content_hash,merkle_root,actual_file_hash,mimetype,write_marker,thumbnail_hash, actual_thumbnail_hash)
VALUES (1234,'exampleId','exampleId:examplePath','exampleId:examplePath','d','root','/','someHash','customMeta','contentHash','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash');
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func GeneratePubPrivateKey(t *testing.T) (pubKey, privateKey string, signScheme zcncrypto.SignatureScheme) {
	signScheme = zcncrypto.NewSignatureScheme("bls0chain")
	wallet, err := signScheme.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	keyPair := wallet.Keys[0]

	_ = signScheme.SetPrivateKey(keyPair.PrivateKey)
	return keyPair.PublicKey, keyPair.PrivateKey, signScheme
}

func setupIntegrationTestConfig(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	configDir := strings.Split(pwd, "/code/go")[0] + "/config"
	config.SetupDefaultConfig()
	config.SetupConfig(configDir)

	config.Configuration.DBHost = "localhost"
	config.Configuration.DBName = viper.GetString("db.name")
	config.Configuration.DBPort = viper.GetString("db.port")
	config.Configuration.DBUserName = viper.GetString("db.user")
	config.Configuration.DBPassword = viper.GetString("db.password")
}

func (c *TestDataController) AddCommitTestData(allocationTx, pubkey, clientId, wmSig string, now common.Timestamp) error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, blobber_size, allocation_root, repairer_id, is_immutable)
VALUES ('exampleId' ,'` + allocationTx + `','` + clientId + `','` + pubkey + `',` + fmt.Sprint(expTime) + `, 99999999, '/', 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO allocation_connections (id, allocation_id, client_id, size, status)
VALUES ('connection_id' ,'exampleId','` + clientId + `', 1337, 1);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO allocation_changes (id, connection_id, operation, size, input)
VALUES (1 ,'connection_id','rename', 1200, '{"allocation_id":"exampleId","path":"/some_file","new_name":"new_name"}');
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO write_markers(prev_allocation_root, allocation_root, status, allocation_id, size, client_id, signature, blobber_id, timestamp, connection_id, client_key)
VALUES ('/', '/', 2,'exampleId', 1337, '` + clientId + `','` + wmSig + `','blobber_id', ` + fmt.Sprint(now) + `, 'connection_id', '` + pubkey + `');
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO reference_objects (id, allocation_id, path_hash,lookup_hash,type,name,path,hash,custom_meta,content_hash,merkle_root,actual_file_hash,mimetype,write_marker,thumbnail_hash, actual_thumbnail_hash, parent_path)
VALUES 
(1234,'exampleId','exampleId:examplePath','exampleId:examplePath','d','root','/','someHash','customMeta','contentHash','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash','/'),
(123,'exampleId','exampleId:examplePath','exampleId:examplePath','f','some_file','/some_file','someHash','customMeta','contentHash','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash','/');
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *TestDataController) AddCopyObjectData(allocationTx, pubkey, clientId string) error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, blobber_size, allocation_root, repairer_id, is_immutable)
VALUES ('exampleId' ,'` + allocationTx + `','` + clientId + `','` + pubkey + `',` + fmt.Sprint(expTime) + `, 99999999, '/', 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO allocation_connections (id, allocation_id, client_id, size, status)
VALUES ('connection_id' ,'exampleId','` + clientId + `', 1337, 1);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO reference_objects (id, allocation_id, path_hash,lookup_hash,type,name,path,hash,custom_meta,content_hash,merkle_root,actual_file_hash,mimetype,write_marker,thumbnail_hash, actual_thumbnail_hash, parent_path)
VALUES 
(1234,'exampleId','exampleId:examplePath','exampleId:examplePath','d','root','/copy','someHash','customMeta','contentHash','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash','/'),
(123,'exampleId','exampleId:examplePath','exampleId:examplePath','f','some_file','/some_file','someHash','customMeta','contentHash','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash','/');
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *TestDataController) AddRenameTestData(allocationTx, pubkey, clientId string) error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, blobber_size, allocation_root, repairer_id, is_immutable)
VALUES ('exampleId' ,'` + allocationTx + `','` + clientId + `','` + pubkey + `',` + fmt.Sprint(expTime) + `, 99999999, '/', 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO allocation_connections (id, allocation_id, client_id, size, status)
VALUES ('connection_id' ,'exampleId','` + clientId + `', 1337, 1);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO reference_objects (id, allocation_id, path_hash,lookup_hash,type,name,path,hash,custom_meta,content_hash,merkle_root,actual_file_hash,mimetype,write_marker,thumbnail_hash, actual_thumbnail_hash, parent_path)
VALUES 
(1234,'exampleId','exampleId:examplePath','exampleId:examplePath','d','root','/','someHash','customMeta','contentHash','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash','/'),
(123,'exampleId','exampleId:examplePath','exampleId:examplePath','f','some_file','/some_file','someHash','customMeta','contentHash','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash','/');
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *TestDataController) AddDownloadTestData(allocID, allocTx, pathHash, contentHash, pubkey, clientId, wmSig string, now common.Timestamp) error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, blobber_size, allocation_root, repairer_id, is_immutable)
VALUES ('` + allocID + `' ,'` + allocTx + `','` + clientId + `','` + pubkey + `',` + fmt.Sprint(expTime) + `, 99999999, '/', 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO allocation_connections (id, allocation_id, client_id, size, status)
VALUES ('connection_id' ,'` + allocID + `','` + clientId + `', 1337, 1);
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO allocation_changes (id, connection_id, operation, size, input)
VALUES (1 ,'connection_id','rename', 1200, '{"allocation_id":"` + allocID + `","path":"/some_file","new_name":"new_name"}');
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
INSERT INTO reference_objects (id, allocation_id, path_hash,lookup_hash,type,name,path,hash,custom_meta,content_hash,merkle_root,actual_file_hash,mimetype,write_marker,thumbnail_hash, actual_thumbnail_hash, parent_path)
VALUES 
(1234,'` + allocID + `','` + pathHash + `','` + pathHash + `','d','root','/','someHash','customMeta','` + contentHash + `','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash','/'),
(123,'` + allocID + `','` + pathHash + `','` + pathHash + `','f','some_file','/some_file','someHash','customMeta','` + contentHash + `','merkleRoot','actualFileHash','mimetype','writeMarker','thumbnailHash','actualThumbnailHash','/');
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *TestDataController) AddUploadTestData(allocationTx, pubkey, clientId string) error {
	var err error
	var tx *sql.Tx
	defer func() {
		if err != nil {
			if tx != nil {
				errRollback := tx.Rollback()
				if errRollback != nil {
					log.Println(errRollback)
				}
			}
		}
	}()

	db, err := c.db.DB()
	if err != nil {
		return err
	}

	tx, err = db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	expTime := time.Now().Add(time.Hour * 100000).UnixNano()

	_, err = tx.Exec(`
INSERT INTO allocations (id, tx, owner_id, owner_public_key, expiration_date, blobber_size, allocation_root, repairer_id, is_immutable)
VALUES ('exampleId' ,'` + allocationTx + `','` + clientId + `','` + pubkey + `',` + fmt.Sprint(expTime) + `, 99999999, '/', 'repairer_id', false);
`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
