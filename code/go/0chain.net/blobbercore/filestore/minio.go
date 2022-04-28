package filestore

import (
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/minio/minio-go"
)

func initializeMinio() (mc *minio.Client, bucket string, err error) {
	if !config.Configuration.MinioStart {
		logging.Logger.Info("Skipping minio start")
		return nil, "", nil
	}

	mc, err = minio.New(
		config.Configuration.MinioStorageUrl,
		config.Configuration.MinioAccessID,
		config.Configuration.MinioSecretKey,
		config.Configuration.MinioUseSSL,
	)
	if err != nil {
		return
	}

	bucket = config.Configuration.MinioBucket
	region := config.Configuration.MinioRegion

	logging.Logger.Info(fmt.Sprintf("Checking if bucket %s exists", bucket))
	isExist, err := mc.BucketExists(bucket)
	switch {
	case isExist:
		logging.Logger.Info("Bucket exists")
	case err != nil:
		return
	default:
		logging.Logger.Info("Bucket does not exist. Creating bucket")
		err = mc.MakeBucket(bucket, region)
		if err != nil {
			return
		}
	}

	return
}

func (fs *FileStore) MinioUpload(fileHash, filePath string) (err error) {
	if fs.mc != nil {
		_, err = fs.mc.FPutObject(fs.bucket, fileHash, filePath, minio.PutObjectOptions{})
		return
	}
	return common.NewError("minio_client_not_set", "")
}

func (fs *FileStore) MinioDownload(fileHash, filePath string) error {
	if fs.mc != nil {
		return fs.mc.FGetObject(fs.bucket, fileHash, filePath, minio.GetObjectOptions{})
	}
	return common.NewError("minio_client_not_set", "")
}

func (fs *FileStore) MinioDelete(fileHash string) error {
	if fs.mc != nil {
		_, err := fs.mc.StatObject(fs.bucket, fileHash, minio.StatObjectOptions{})
		if err == nil {
			return fs.mc.RemoveObject(fs.bucket, fileHash)
		}
		return nil
	}
	return common.NewError("minio_client_not_set", "")
}

func (fs *FileStore) MinioGet(fileHash string) (minio.ObjectInfo, error) {
	if fs.mc != nil {
		return fs.mc.StatObject(fs.bucket, fileHash, minio.StatObjectOptions{})
	}
	return minio.ObjectInfo{}, common.NewError("minio_client_not_set", "")
}
