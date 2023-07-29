package handler

import (
	"encoding/base64"
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

// LoadPlaylist load latest playlist
func LoadPlaylist(ctx *Context) (interface{}, error) {
	q := ctx.Request.URL.Query()

	since := q.Get("since")

	authTokenString := q.Get("auth_token")

	//load playlist with auth ticket
	if len(authTokenString) > 0 {

		lookupHash := q.Get("lookup_hash")

		if len(lookupHash) == 0 {
			return nil, errors.New("lookup_hash_missed: auth_token and lookup_hash are required")
		}

		fileRef, err := reference.GetLimitedRefFieldsByLookupHashWith(ctx, ctx.AllocationId, lookupHash, []string{"id", "path", "lookup_hash", "type", "name"})
		if err != nil {
			return nil, common.NewError("invalid_lookup_hash", err.Error())
		}

		at, err := base64.StdEncoding.DecodeString(authTokenString)
		if err != nil {
			return nil, common.NewError("invalid_auth_ticket", err.Error())
		}

		authToken, err := verifyAuthTicket(ctx, string(at), ctx.Allocation, fileRef, ctx.ClientID, true)
		if err != nil {
			return nil, err
		}
		if authToken == nil {
			return nil, common.NewError("auth_ticket_verification_failed", "Could not verify the auth ticket.")
		}

		return reference.LoadPlaylist(ctx, ctx.AllocationId, fileRef.Path, since)

	}

	if ctx.ClientID == "" || ctx.ClientID != ctx.Allocation.OwnerID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	return reference.LoadPlaylist(ctx, ctx.AllocationId, q.Get("path"), since)
}

// LoadPlaylistFile load playlist file
func LoadPlaylistFile(ctx *Context) (interface{}, error) {
	q := ctx.Request.URL.Query()

	lookupHash := q.Get("lookup_hash")
	if len(lookupHash) == 0 {
		return nil, errors.New("lookup_hash_missed: lookup_hash is required")
	}

	authTokenString := q.Get("auth_token")

	//load playlist with auth ticket
	if len(authTokenString) > 0 {
		fileRef, err := reference.GetLimitedRefFieldsByLookupHashWith(ctx, ctx.AllocationId, lookupHash, []string{"id", "path", "lookup_hash", "type", "name"})
		if err != nil {
			return nil, common.NewError("invalid_lookup_hash", err.Error())
		}
		at, err := base64.StdEncoding.DecodeString(authTokenString)
		if err != nil {
			return nil, common.NewError("invalid_auth_ticket", err.Error())
		}
		authToken, err := verifyAuthTicket(ctx, string(at), ctx.Allocation, fileRef, ctx.ClientID, true)
		if err != nil {
			return nil, err
		}
		if authToken == nil {
			return nil, common.NewError("auth_ticket_verification_failed", "Could not verify the auth ticket.")
		}

		return reference.LoadPlaylistFile(ctx, ctx.AllocationId, lookupHash)

	}

	if ctx.ClientID == "" || ctx.ClientID != ctx.Allocation.OwnerID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	return reference.LoadPlaylistFile(ctx, ctx.AllocationId, lookupHash)
}
