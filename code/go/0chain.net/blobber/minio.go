package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

func setupMinio() error {
	fmt.Print("> setup minio")

	if config.Configuration.MinioStart {
		fmt.Print("	+ No minio 	[SKIP]\n")
		return nil
	}

	reader, err := os.Open(minioFile)
	if err != nil {
		return err
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	more := scanner.Scan()
	if !more {
		return common.NewError("process_minio_config_failed", "Unable to read minio config from minio config file")
	}

	filestore.MinioConfig.StorageServiceURL = scanner.Text()
	more = scanner.Scan()
	if !more {
		return common.NewError("process_minio_config_failed", "Unable to read minio config from minio config file")
	}

	filestore.MinioConfig.AccessKeyID = scanner.Text()
	more = scanner.Scan()
	if !more {
		return common.NewError("process_minio_config_failed", "Unable to read minio config from minio config file")
	}

	filestore.MinioConfig.SecretAccessKey = scanner.Text()
	more = scanner.Scan()
	if !more {
		return common.NewError("process_minio_config_failed", "Unable to read minio config from minio config file")
	}

	filestore.MinioConfig.BucketName = scanner.Text()
	more = scanner.Scan()
	if !more {
		return common.NewError("process_minio_config_failed", "Unable to read minio config from minio config file")
	}

	filestore.MinioConfig.BucketLocation = scanner.Text()

	fmt.Print("		[OK]\n")
	return nil
}
