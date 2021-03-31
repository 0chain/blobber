package handler

import (
	"0chain.net/blobbercore/constants"
	"0chain.net/core/config"
	"0chain.net/core/logging"
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestDoNotThrowInvalidSignatureErrorWhenSignatureValidForGivenRequestAndContext(t *testing.T) {
	logging.InitLogging("development", "", "")
	var blobber = &StorageHandler{}
	var httpReq = &http.Request{}
	httpReq.Form = url.Values{}
	var ctx = httpReq.Context()

	config.Configuration.SignatureScheme = "bls0chain"
	httpReq.Form.Add("signature", "471e621e14f9cdb5acaeaebb42decc90be7a66852e851c89dc4d4ca857426d97")
	httpReq.Form.Add("auth_token", "valid_auth_token")
	httpReq.Form.Add("path", "expected_path")
	httpReq.Form.Add("path_hash", "expected_haah")
	ctx = context.WithValue(ctx, constants.CLIENT_CONTEXT_KEY, "valid_id")
	ctx = context.WithValue(ctx, constants.CLIENT_KEY_CONTEXT_KEY, "de7c0361d75aa102821cba93863bdda39bae8aab94030130a61f660f2fb263049919cb8450d3772c8638a5d99f28497d53d6a6a01ae89865d4ec50405d1be380")

	signatureValid := false
	defer func() {
		if err := recover(); err != nil {
			signatureValid = true
		}
	}()
	blobber.GetFileMeta(ctx, httpReq)

	if !signatureValid {
		t.Errorf("GetFileMeta() = expected vaid signature but was invalid")
	}
}

func TestThrowInvalidSignatureErrorWhenSignatureInvalidForGivenRequestAndContextTooShort(t *testing.T) {
	var blobber = &StorageHandler{}
	var httpReq = &http.Request{}
	httpReq.Form = url.Values{}
	var ctx = httpReq.Context()

	httpReq.Form.Add("signature", "invalid_signature")
	httpReq.Form.Add("auth_token", "valid_auth_token")
	httpReq.Form.Add("path", "expected_path")
	httpReq.Form.Add("path_hash", "expected_haah")
	ctx = context.WithValue(ctx, constants.CLIENT_CONTEXT_KEY, "valid_id")
	ctx = context.WithValue(ctx, constants.CLIENT_KEY_CONTEXT_KEY, "e49946289899ee764f34f2c8890b37a481a431586f9a5dcb4cc4a85418e8820ff09a919ed6bb6afa255240859f7116c341907d16c3455489fbbca11788854e05")
	_, err := blobber.GetFileMeta(ctx, httpReq)

	var expectedErr = "invalid_parameters: Invalid Signature"
	if err == nil {
		t.Errorf("GetFileMeta() = expected error  but was %v", nil)
	} else if err.Error() != expectedErr {
		t.Errorf("GetFileMeta() = expected error message to be %v  but was %v", expectedErr, err.Error())
	}
}

func TestThrowInvalidSignatureErrorWhenSignatureInvalidForGivenRequestAndContext(t *testing.T) {
	var blobber = &StorageHandler{}
	var httpReq = &http.Request{}
	httpReq.Form = url.Values{}
	var ctx = httpReq.Context()

	httpReq.Form.Add("signature", "471e621e14f9cdb5acaeaebb42decc90be7a66852e851c89dc4d4ca857426d98")
	httpReq.Form.Add("auth_token", "valid_auth_token")
	httpReq.Form.Add("path", "expected_path")
	httpReq.Form.Add("path_hash", "expected_haah")
	ctx = context.WithValue(ctx, constants.CLIENT_CONTEXT_KEY, "valid_id")
	ctx = context.WithValue(ctx, constants.CLIENT_KEY_CONTEXT_KEY, "e49946289899ee764f34f2c8890b37a481a431586f9a5dcb4cc4a85418e8820ff09a919ed6bb6afa255240859f7116c341907d16c3455489fbbca11788854e05")
	_, err := blobber.GetFileMeta(ctx, httpReq)

	var expectedErr = "invalid_parameters: Invalid Signature"
	if err == nil {
		t.Errorf("GetFileMeta() = expected error  but was %v", nil)
	} else if err.Error() != expectedErr {
		t.Errorf("GetFileMeta() = expected error message to be %v  but was %v", expectedErr, err.Error())
	}
}

func TestThrowInvalidSignatureErrorWhenSignatureNotPresentForGivenRequestAndContext(t *testing.T) {
	var blobber = &StorageHandler{}
	var httpReq = &http.Request{}
	httpReq.Form = url.Values{}
	var ctx = httpReq.Context()

	ctx = context.WithValue(ctx, constants.CLIENT_CONTEXT_KEY, "")
	ctx = context.WithValue(ctx, constants.CLIENT_KEY_CONTEXT_KEY, "")
	_, err := blobber.GetFileMeta(ctx, httpReq)

	var expectedErr = "invalid_parameters: Invalid Signature"
	if err == nil {
		t.Errorf("GetFileMeta() = expected error  but was %v", nil)
	} else if err.Error() != expectedErr {
		t.Errorf("GetFileMeta() = expected error message to be %v  but was %v", expectedErr, err.Error())
	}
}
