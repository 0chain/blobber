package handler

import (
	"encoding/base64"
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

// swagger:route GET /v1/file/playlist/{allocation} GetPlaylist
// Get playlist.
// Loads playlist from a given path in an allocation.
//
// parameters:
//   +name: allocation
//     in: path
//     type: string
//     required: true
//     description: allocation id
//	 +name: X-App-Client-ID
//     description: The ID/Wallet address of the client sending the request.
//     in: header
//     type: string
//     required: true
//	 +name: X-App-Client-Key
// 	   description: The key of the client sending the request.
//     in: header
//     type: string
//     required: true
//	 +name: ALLOCATION-ID
//	   description: The ID of the allocation in question.
//     in: header
//     type: string
//     required: true
//  +name: X-App-Client-Signature
//     description: Digital signature of the client used to verify the request if the X-Version is not "v2"
//     in: header
//     type: string
//  +name: X-App-Client-Signature-V2
//     description: Digital signature of the client used to verify the request if the X-Version is "v2"
//     in: header
//     type: string
//  +name: since
//     description: The lookup hash of the file from which to start the playlist. The retrieved playlist will start from the id associated with this lookup hash and going forward.
//     in: query
//     type: string
//     required: false
//  +name: auth_token
//     description: The auth token to access the playlist. This is required when the playlist is accessed by a non-owner of the allocation.
//     in: query
//     type: string
//     required: false
//  +name: lookup_hash
//     description: The lookup hash of the file for which the playlist is to be retrieved. This is required when the playlist is accessed by a non-owner of the allocation.
//     in: query
//     type: string
//     required: false
//  +name: path
//     description: The path of the file for which the playlist is to be retrieved. This is required when the playlist is accessed by the owner of the allocation.
//     in: query
//     type: string
//     required: false
//
// responses:
//   200: []PlaylistFile
//   400:
//   500:
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

// swagger:route GET /v1/playlist/file/{allocation} GetPlaylistFile
// Get playlist file.
// Loads the metadata of a the playlist file with the given lookup hash.
//
// parameters:
//   +name: allocation
//     in: path
//     type: string
//     required: true
//     description: allocation id
//	 +name: X-App-Client-ID
//     description: The ID/Wallet address of the client sending the request.
//     in: header
//     type: string
//     required: true
//	 +name: X-App-Client-Key
// 	   description: The key of the client sending the request.
//     in: header
//     type: string
//     required: true
//	 +name: ALLOCATION-ID
//	   description: The ID of the allocation in question.
//     in: header
//     type: string
//     required: true
//  +name: X-App-Client-Signature
//     description: Digital signature of the client used to verify the request if the X-Version is not "v2"
//     in: header
//     type: string
//  +name: X-App-Client-Signature-V2
//     description: Digital signature of the client used to verify the request if the X-Version is "v2"
//     in: header
//     type: string
//  +name: auth_token
//     description: The auth token to access the playlist. This is required when the playlist is accessed by a non-owner of the allocation.
//     in: query
//     type: string
//     required: false
//  +name: lookup_hash
//     description: The lookup hash of the file for which the playlist is to be retrieved.
//     in: query
//     type: string
//     required: false
//
// responses:
//   200: PlaylistFile
//   400:
//   500:
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
