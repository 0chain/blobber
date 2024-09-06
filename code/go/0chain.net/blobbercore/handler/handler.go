package handler

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/0chain/gosdk/core/zcncrypto"

	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"

	"github.com/go-openapi/runtime/middleware"

	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"github.com/didip/tollbooth/v6/limiter"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

const (
	CommitRPS  = 5 // Commit Request Per Second
	FileRPS    = 5 // File Request Per Second
	ObjectRPS  = 5 // Object Request Per Second
	GeneralRPS = 5 // General Request Per Second

	DefaultExpirationTTL = time.Minute * 5
)

var (
	commitRL  *limiter.Limiter // commit Rate Limiter
	fileRL    *limiter.Limiter // file Rate Limiter
	objectRL  *limiter.Limiter // object Rate Limiter
	generalRL *limiter.Limiter // general Rate Limiter
)

var storageHandler StorageHandler

func GetMetaDataStore() datastore.Store {
	return datastore.GetStore()
}

func ConfigRateLimits() {
	tokenExpirettl := viper.GetDuration("rate_limiters.default_token_expire_duration")
	if tokenExpirettl <= 0 {
		tokenExpirettl = DefaultExpirationTTL
	}

	ipLookups := []string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"}

	isProxy := viper.GetBool("rate_limiters.proxy")
	if isProxy {
		ipLookups = []string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"}
	}

	cRps := viper.GetFloat64("rate_limiters.commit_rps")
	fRps := viper.GetFloat64("rate_limiters.file_rps")
	oRps := viper.GetFloat64("rate_limiters.object_rps")
	gRps := viper.GetFloat64("rate_limiters.general_rps")

	if cRps <= 0 {
		cRps = CommitRPS
	}

	if fRps <= 0 {
		fRps = FileRPS
	}

	if oRps <= 0 {
		oRps = ObjectRPS
	}

	if gRps <= 0 {
		gRps = GeneralRPS
	}

	Logger.Info("Setting rps: ",
		zap.Float64("commit_rps", cRps),
		zap.Float64("file_rps", fRps),
		zap.Float64("object_rps", oRps),
		zap.Float64("general_rps", gRps),
	)

	commitRL = common.GetRateLimiter(cRps, ipLookups, true, tokenExpirettl)
	fileRL = common.GetRateLimiter(fRps, ipLookups, true, tokenExpirettl)
	objectRL = common.GetRateLimiter(oRps, ipLookups, true, tokenExpirettl)
	generalRL = common.GetRateLimiter(gRps, ipLookups, true, tokenExpirettl)
}

func RateLimitByFileRL(handler common.ReqRespHandlerf) common.ReqRespHandlerf {
	return common.RateLimit(handler, fileRL)
}

func RateLimitByCommmitRL(handler common.ReqRespHandlerf) common.ReqRespHandlerf {
	return common.RateLimit(handler, commitRL)
}

func RateLimitByObjectRL(handler common.ReqRespHandlerf) common.ReqRespHandlerf {
	return common.RateLimit(handler, objectRL)
}

func RateLimitByGeneralRL(handler common.ReqRespHandlerf) common.ReqRespHandlerf {
	return common.RateLimit(handler, generalRL)
}

func SetupSwagger() {
	http.Handle("/swagger.yaml", http.FileServer(http.Dir("/docs")))

	// documentation for developers
	opts := middleware.SwaggerUIOpts{SpecURL: "swagger.yaml"}
	sh := middleware.SwaggerUI(opts, nil)
	http.Handle("/docs", sh)

	// documentation for share
	opts1 := middleware.RedocOpts{SpecURL: "swagger.yaml", Path: "docs1"}
	sh1 := middleware.Redoc(opts1, nil)
	http.Handle("/docs1", sh1)
}

func WithBlobberRegistered(h http.Handler) http.Handler {
	if !common.IsBlobberRegistered() {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("Blobber not registered yet"))
		})
	}

	return h
}

/*setupHandlers sets up the necessary API end points */
func setupHandlers(s *mux.Router) {
	ConfigRateLimits()

	s.Use(UseRecovery, UseCors, WithBlobberRegistered)

	s.HandleFunc("/_stats", RateLimitByCommmitRL(StatsHandler))

	//object operations
	s.HandleFunc("/v1/connection/create/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(CreateConnectionHandler)))).
		Methods(http.MethodPost)

	s.HandleFunc("/v1/connection/redeem/{allocation}",
		RateLimitByGeneralRL(common.ToByteStream(WithConnection(RedeemHandler)))).
		Methods(http.MethodPost, http.MethodOptions)

	s.HandleFunc("/v1/file/rename/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(RenameHandler)))).
		Methods(http.MethodPost, http.MethodOptions)

	s.HandleFunc("/v1/file/copy/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(CopyHandler)))).
		Methods(http.MethodPost, http.MethodOptions)

	s.HandleFunc("/v1/file/move/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(MoveHandler)))).
		Methods(http.MethodPost, http.MethodOptions)

	s.HandleFunc("/v1/dir/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(CreateDirHandler)))).
		Methods(http.MethodPost, http.MethodDelete, http.MethodOptions)

	s.HandleFunc("/v1/connection/commit/{allocation}",
		RateLimitByCommmitRL(common.ToStatusCode(WithStatusConnectionForWM(CommitHandler))))

	s.HandleFunc("/v2/connection/commit/{allocation}",
		RateLimitByCommmitRL(common.ToStatusCode(WithStatusConnectionForWM(CommitHandlerV2))))

	s.HandleFunc("/v1/connection/rollback/{allocation}",
		RateLimitByCommmitRL(common.ToStatusCode(WithStatusConnectionForWM(RollbackHandler))))

	//object info related apis
	s.HandleFunc("/allocation",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(AllocationHandler))))

	s.HandleFunc("/v1/file/meta/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithReadOnlyConnection(FileMetaHandler))))

	s.HandleFunc("/v1/file/stats/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithReadOnlyConnection(FileStatsHandler))))

	s.HandleFunc("/v1/file/referencepath/{allocation}",
		RateLimitByObjectRL(common.ToJSONResponse(WithReadOnlyConnection(ReferencePathHandler))))

	s.HandleFunc("/v2/file/referencepath/{allocation}",
		RateLimitByObjectRL(common.ToJSONResponse(WithReadOnlyConnection(ReferencePathV2Handler))))

	s.HandleFunc("/v1/file/latestwritemarker/{allocation}",
		RateLimitByObjectRL(common.ToJSONResponse(WithReadOnlyConnection(WriteMarkerHandler))))

	s.HandleFunc("/v1/file/objecttree/{allocation}",
		RateLimitByObjectRL(common.ToStatusCode(WithStatusReadOnlyConnection(ObjectTreeHandler)))).
		Methods(http.MethodGet, http.MethodOptions)

	s.HandleFunc("/v1/file/refs/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithReadOnlyConnection(RefsHandler)))).
		Methods(http.MethodGet, http.MethodOptions)

	s.HandleFunc("/v1/file/refs/recent/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithReadOnlyConnection(RecentRefsRequestHandler)))).
		Methods(http.MethodGet, http.MethodOptions)

	// admin related
	// Allowing admin api for debugging purpose only. Later on commented out line should be
	// uncommented and line below it should be deleted

	s.HandleFunc("/_debug", common.AuthenticateAdmin(common.ToJSONResponse(DumpGoRoutines)))
	// s.HandleFunc("/_debug", RateLimitByCommmitRL(common.ToJSONResponse(DumpGoRoutines)))
	s.HandleFunc("/_config", common.AuthenticateAdmin(common.ToJSONResponse(GetConfig)))
	// s.HandleFunc("/_config", RateLimitByCommmitRL(common.ToJSONResponse(GetConfig)))
	// s.HandleFunc("/_stats", common.AuthenticateAdmin(StatsHandler)))
	s.HandleFunc("/objectlimit", RateLimitByCommmitRL(common.ToJSONResponse(GetObjectLimit)))

	s.HandleFunc("/_logs", RateLimitByCommmitRL(common.ToJSONResponse(GetLogs)))

	// s.HandleFunc("/_cleanupdisk", common.AuthenticateAdmin(common.ToJSONResponse(WithReadOnlyConnection(CleanupDiskHandler)))))
	// s.HandleFunc("/_cleanupdisk", RateLimitByCommmitRL(common.ToJSONResponse(WithReadOnlyConnection(CleanupDiskHandler)))))
	s.HandleFunc("/challengetimings", common.AuthenticateAdmin(common.ToJSONResponse(GetChallengeTimings)))
	// s.HandleFunc("/challengetimings", RateLimitByCommmitRL(common.ToJSONResponse(GetChallengeTimings)))
	s.HandleFunc("/challenge-timings-by-challengeId", RateLimitByCommmitRL(common.ToJSONResponse(GetChallengeTiming)))

	// Generate auth ticket
	s.HandleFunc("/v1/auth/generate", Authenticate0Box(common.ToJSONResponse(GenerateAuthTicket)))

	//marketplace related
	s.HandleFunc("/v1/marketplace/shareinfo/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(InsertShare)))).
		Methods(http.MethodOptions, http.MethodPost)

	s.HandleFunc("/v1/marketplace/shareinfo/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(RevokeShare)))).
		Methods(http.MethodOptions, http.MethodDelete)

	// list files shared in this allocation
	s.HandleFunc("/v1/marketplace/shareinfo/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(ListShare)))).
		Methods(http.MethodOptions, http.MethodGet)

	// lightweight http handler without heavy postgres transaction to improve performance

	s.HandleFunc("/v1/writemarker/lock/{allocation}",
		RateLimitByGeneralRL(WithTxHandler(LockWriteMarker))).
		Methods(http.MethodPost, http.MethodOptions)

	s.HandleFunc("/v1/writemarker/lock/{allocation}/{connection}",
		RateLimitByGeneralRL(WithTxHandler(UnlockWriteMarker))).
		Methods(http.MethodDelete, http.MethodOptions)

	// TODO: Deprecated, remove in future
	s.HandleFunc("/v1/hashnode/root/{allocation}",
		RateLimitByObjectRL(WithTxHandler(LoadRootHashnode))).
		Methods(http.MethodGet, http.MethodOptions)

	s.HandleFunc("/v1/playlist/latest/{allocation}",
		RateLimitByGeneralRL(WithTxHandler(LoadPlaylist))).
		Methods(http.MethodGet, http.MethodOptions)

	s.HandleFunc("/v1/playlist/file/{allocation}",
		RateLimitByGeneralRL(WithTxHandler(LoadPlaylistFile))).
		Methods(http.MethodGet, http.MethodOptions)

}

func WithReadOnlyConnection(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = GetMetaDataStore().CreateTransaction(ctx, &sql.TxOptions{
			ReadOnly: true,
		})
		tx := GetMetaDataStore().GetTransaction(ctx)
		defer func() {
			tx.Rollback()
		}()

		res, err := handler(ctx, r)
		return res, err
	}
}

func WithConnection(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		var (
			resp interface{}
			err  error
		)
		err = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
			resp, err = handler(ctx, r)

			return err
		})
		return resp, err
	}
}

func Authenticate0Box(handler common.ReqRespHandlerf) common.ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {

		signature := r.Header.Get("Zbox-Signature")
		if signature == "" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Invalid signature")) // nolint
			return
		}

		signatureScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
		if err := signatureScheme.SetPublicKey(common.PublicKey0box); err != nil {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Invalid signature")) // nolint
			return
		}

		success, err := signatureScheme.Verify(signature, hex.EncodeToString([]byte(common.PublicKey0box)))
		if err != nil || !success {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Invalid signature")) // nolint
			return
		}

		handler(w, r)
	}
}

func setupHandlerContext(ctx context.Context, r *http.Request) context.Context {
	var vars = mux.Vars(r)

	ctx = context.WithValue(ctx, constants.ContextKeyClient,
		r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, constants.ContextKeyClientKey,
		r.Header.Get(common.ClientKeyHeader))
	ctx = context.WithValue(ctx, constants.ContextKeyAllocation,
		vars["allocation"])

	ctx = context.WithValue(ctx, constants.ContextKeyAllocationID, r.Header.Get(common.AllocationIdHeader))

	// signature is not requered for all requests, but if header is empty it won`t affect anything
	ctx = context.WithValue(ctx, constants.ContextKeyClientSignatureHeaderKey, r.Header.Get(common.ClientSignatureHeader))
	// signature V2
	ctx = context.WithValue(ctx, constants.ContextKeyClientSignatureHeaderV2Key, r.Header.Get(common.ClientSignatureHeaderV2))
	return ctx
}

// swagger:route GET /allocation GetAllocation
// Get allocation details.
// Retrieve allocation details as stored in the blobber.
//
// parameters:
//	 +name: id
//	   description: allocation ID
//	   required: true
//	   in: query
//	   type: string
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
//
// responses:
//
//	200: Allocation
//	400:
//	500:

func AllocationHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetAllocationDetails(ctx, r)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// swagger:route GET /v1/file/meta/{allocation} GetFileMeta
// Get file meta data.
// Retrieve file meta data from the blobber. Retrieves a generic map of string keys and values.
//
// parameters:
//
//		 +name: allocation
//		   description: the allocation ID
//		   required: true
//		   in: path
//		   type: string
//		 +name: X-App-Client-ID
//	    description: The ID/Wallet address of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		 +name: X-App-Client-Key
//		   description: The key of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		 +name: ALLOCATION-ID
//		   description: The ID of the allocation in question.
//	    in: header
//	    type: string
//	    required: true
//	 +name: X-App-Client-Signature
//	    description: Digital signature of the client used to verify the request if the X-Version is not "v2"
//	    in: header
//	    type: string
//	 +name: X-App-Client-Signature-V2
//	    description: Digital signature of the client used to verify the request if the X-Version is "v2"
//	    in: header
//	    type: string
//	 +name: name
//	    description: the name of the file
//	    required: false
//	    in: query
//	    type: string
//	 +name: path
//	    description: Path of the file to be copied. Required only if `path_hash` is not provided.
//	    in: query
//	    type: string
//	 +name: path_hash
//	    description: Hash of the path of the file to be copied. Required only if `path` is not provided.
//	    in: query
//	    type: string
//	 +name: auth_token
//	    description: The auth ticket for the file to show meta data of if the client does not own it. Check File Sharing docs for more info.
//	    in: query
//	    type: string
//	    required: false
//
// responses:
//
//	200:
//	400:
//	500:
func FileMetaHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)

	name := r.FormValue("name")
	if strings.TrimSpace(name) != "" {
		response, err := storageHandler.GetFilesMetaByName(ctx, r, name)
		if err != nil {
			return nil, err
		}
		return response, nil
	}
	response, err := storageHandler.GetFileMeta(ctx, r)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// swagger:route GET /v1/file/stats/{allocation} GetFileStats
// Get file stats.
// Retrieve file stats from the blobber.
//
// parameters:
//
//  +name: allocation
//     description: the allocation ID
//     required: true
//     in: path
//     type: string
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
//  +name: path
//     description: Path of the file to be copied. Required only if `path_hash` is not provided.
//     in: query
//     type: string
//  +name: path_hash
//     description: Hash of the path of the file to be copied. Required only if `path` is not provided.
//     in: query
//     type: string
//
// responses:
//
//	200: FileStats
//	400:
//	500:

func FileStatsHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.GetFileStats(ctx, r)
	if err != nil {
		return nil, err
	}
	return response, nil
}

/*downloadHandler is the handler to respond to download requests from clients*/
func downloadHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)
	return storageHandler.DownloadFile(ctx, r)
}

// swagger:route POST /v1/connection/redeem/{allocation} PostRedeem
// Redeem conncetion.
// Submit the connection ID to redeem the storage cost from the network.
//
// parameters:
//
//		+name: allocation
//		   description: the allocation ID
//		   required: true
//		   in: path
//		   type: string
//		+name: X-App-Client-ID
//	    description: The ID/Wallet address of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		+name: X-App-Client-Key
//		   description: The key of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		+name: ALLOCATION-ID
//		   description: The ID of the allocation in question.
//	    in: header
//	    type: string
//	    required: true
//	 +name: X-App-Client-Signature
//	    description: Digital signature of the client used to verify the request if the X-Version is not "v2"
//	    in: header
//	    type: string
//	 +name: X-App-Client-Signature-V2
//	    description: Digital signature of the client used to verify the request if the X-Version is "v2"
//	    in: header
//	    type: string
//
// responses:
//
//	200: DownloadResponse
//	400:
//	500:
func redeemHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)
	resp, err := storageHandler.RedeemReadMarker(ctx, r)
	logging.Logger.Info("redeemHandler", zap.Any("resp", resp), zap.Error(err))
	return resp, err
}

/*listHandler is the handler to respond to list requests from clients*/
func listHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.ListEntities(ctx, r)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// swagger:route POST /v1/connection/create/{allocation} PostConnection
// Store connection in DB.
// Connections are used to distinguish between different storage operations, also to claim reward from the chain using write markers.
//
// parameters:
//
//		 +name: allocation
//		   description: the allocation ID
//		   required: true
//		   in: path
//		   type: string
//		 +name: X-App-Client-ID
//	    description: The ID/Wallet address of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		 +name: X-App-Client-Key
//		   description: The key of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		 +name: ALLOCATION-ID
//		   description: The ID of the allocation in question.
//	    in: header
//	    type: string
//	    required: true
//	 +name: X-App-Client-Signature
//	    description: Digital signature of the client used to verify the request if the X-Version is not "v2"
//	    in: header
//	    type: string
//	 +name: X-App-Client-Signature-V2
//	    description: Digital signature of the client used to verify the request if the X-Version is "v2"
//	    in: header
//	    type: string
//		+name: connection_id
//		   description: the ID of the connection to submit.
//		   required: true
//		   in: query
//		   type: string
//
// responses:
//
//	200: ConnectionResult
//	400:
//	500:
func CreateConnectionHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)
	res, err := storageHandler.CreateConnection(ctx, r)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// swagger:route POST /v1/connection/commit/{allocation} PostCommit
// Commit operation.
// Used to commit the storage operation provided its connection id.
//
// parameters:
//
//	+name: allocation
//	   description: the allocation ID
//	   required: true
//	   in: path
//	   type: string
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
//	+name: connection_id
//	   description: the connection ID of the storage operation to commit
//	   required: true
//	   in: query
//	   type: string
//	+name: write_marker
//	   description: The write marker corresponding to the operation. Write price is used to redeem storage cost from the network. It follows the format of the [Write Marker](#write-marker)
//	   required: true
//	   in: query
//	   type: string
//
// responses:
//
//	200: CommitResult
//	400:
//	500:

func CommitHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	return commitHandler(ctx, r)
}

func CommitHandlerV2(ctx context.Context, r *http.Request) (interface{}, int, error) {
	return commitHandlerV2(ctx, r)
}

// swagger:route POST/v1/connection/rollback/{allocation} PostRollback
// Rollback operation.
// RollbackHandler used to commit the storage operation provided its connection id.
//
// parameters:
//
//	+name: allocation
//	   description: the allocation ID
//	   required: true
//	   in: path
//	   type: string
//	+name: X-App-Client-ID
//     description: The ID/Wallet address of the client sending the request.
//     in: header
//     type: string
//     required: true
//	+name: X-App-Client-Key
// 	   description: The key of the client sending the request.
//     in: header
//     type: string
//     required: true
//	+name: ALLOCATION-ID
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
//	+name: connection_id
//	   description: the connection ID of the storage operation to rollback
//	   required: true
//	   in: query
//	   type: string
//	+name: write_marker
//	   description: The write marker corresponding to the operation. Write price is used to redeem storage cost from the network. It follows the format of the [Write Marker](#write-marker)
//	   required: true
//	   in: query
//	   type: string
//
// responses:
//
//	200: CommitResult
//	400:
//	500:

func RollbackHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	return rollbackHandler(ctx, r)
}

// swagger:route GET /v1/file/referencepath/{allocation} GetReferencePath
// Get reference path.
// Retrieve references of all the decendents of a given path including itself, known as reference path. Reference (shorted as Ref) is the representation of a certain path in the DB including its metadata.
// It also returns the latest write marker associated with the allocation.
//
// parameters:
//
//		+name: allocation
//		   description: the allocation ID
//		   required: true
//		   in: path
//		   type: string
//		+name: X-App-Client-ID
//	    description: The ID/Wallet address of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		+name: X-App-Client-Key
//		   description: The key of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		+name: ALLOCATION-ID
//		   description: The ID of the allocation in question.
//	    in: header
//	    type: string
//	    required: true
//	 +name: X-App-Client-Signature
//	    description: Digital signature of the client used to verify the request if the X-Version is not "v2"
//	    in: header
//	    type: string
//	 +name: X-App-Client-Signature-V2
//	    description: Digital signature of the client used to verify the request if the X-Version is "v2"
//	    in: header
//	    type: string
//	 +name: path
//	   description: Path of the file needed to get reference path of. Required only if no "paths" are provided.
//	   in: query
//	   type: string
//	 +name: paths
//	   description: Paths of the files needed to get reference path of. Required only if no "path" is provided. Should be provided as valid JSON array.
//	   in: query
//	   type: string
//
// responses:
//
//	200: ReferencePathResult
//	400:
//	500:
func ReferencePathHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx, canceler := context.WithTimeout(ctx, time.Second*60)
	defer canceler()

	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetReferencePath(ctx, r)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func ReferencePathV2Handler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx, canceler := context.WithTimeout(ctx, time.Second*60)
	defer canceler()

	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetReferencePathV2(ctx, r)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// swagger:route GET /v1/file/latestwritemarker/{allocation} GetLatestWriteMarker
// Get latest write marker.
// Retrieve the latest write marker associated with the allocation
//
// parameters:
//
//		+name: allocation
//		   description: the allocation ID
//		   required: true
//		   in: path
//		   type: string
//		+name: X-App-Client-ID
//	    description: The ID/Wallet address of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		+name: X-App-Client-Key
//		   description: The key of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		+name: ALLOCATION-ID
//		   description: The ID of the allocation in question.
//	    in: header
//	    type: string
//	    required: true
//	 +name: X-App-Client-Signature
//	    description: Digital signature of the client used to verify the request if the X-Version is not "v2"
//	    in: header
//	    type: string
//	 +name: X-App-Client-Signature-V2
//	    description: Digital signature of the client used to verify the request if the X-Version is "v2"
//	    in: header
//	    type: string
//
// responses:
//
//	200: LatestWriteMarkerResult
//	400:
//	500:
func WriteMarkerHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetLatestWriteMarker(ctx, r)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// swagger:route GET /v1/file/objecttree/{allocation} GetObjectTree
// Get path object tree.
// Retrieve object tree reference path. Similar to reference path.
//
// parameters:
//
//	+name: allocation
//	   description: allocation ID
//	   required: true
//	   in: path
//	   type: string
//	+name: X-App-Client-ID
//     description: The ID/Wallet address of the client sending the request.
//     in: header
//     type: string
//     required: true
//	+name: X-App-Client-Key
// 	   description: The key of the client sending the request.
//     in: header
//     type: string
//     required: true
//	+name: ALLOCATION-ID
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
//  +name: path
//    description: Path of the file needed to get reference path of. Required only if no "paths" are provided.
//    in: query
//    type: string
//
// responses:
//
//	200: ReferencePathResult
//	400:
//	500:

func ObjectTreeHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	return objectTreeHandler(ctx, r)
}

// swagger:route GET /v1/file/refs/{allocation} GetRefs
// Get references.
// Retrieve references of all the decendents of a given path including itself, organized in a paginated table.
//
// parameters:
//
//	+name: allocation
//	   description: allocation ID
//	   required: true
//	   in: path
//	   type: string
//	+name: X-App-Client-ID
//     description: The ID/Wallet address of the client sending the request.
//     in: header
//     type: string
//     required: true
//	+name: X-App-Client-Key
// 	   description: The key of the client sending the request.
//     in: header
//     type: string
//     required: true
//	+name: ALLOCATION-ID
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
//  +name: path
//    description: Path of the file needed to get reference path of. Required only if no "paths" are provided.
//    in: query
//    type: string
//  +name: auth_token
//    description: The auth ticket for the file to show meta data of if the client does not own it. Check File Sharing docs for more info.
//    in: query
//    type: string
//    required: false
//  +name: path_hash
//    description: Hash of the path of the file to be copied. Required only if `path` is not provided.
//    in: query
//    type: string
//  +name: refType
//    description: Can be "updated" (along with providing `updateDate` and `offsetDate`) to retrieve refs with updated_at date later than the provided date in both fields, or "regular" otherwise.
//    in: query
//    type: string
//    required: true
//  +name: pageLimit
//    description: Number of records to show per page. Default is 20.
//    in: query
//    type: integer
//  +name: offsetPath
//    description: Path of the file to start the listing from. Used for pagination.
//    in: query
//    type: string
//  +name: offsetDate
//    description: Date of the file to start the listing from.  Used in case the user needs to list refs updated at some period of time.
//    in: query
//    type: string
//  +name: updateDate
//    description: Same as offsetDate but both should be provided.
//    in: query
//    type: string
//  +name: fileType
//    description: Type of the references to list. Can be "f" for file or "d" for directory. Both will be retrieved if not provided.
//    in: query
//    type: string
//  +name: level
//    description: Level of the references to list (number of parents of the reference). Can be "0" for root level or "1" for first level and so on. All levels will be retrieved if not provided.
//    in: query
//    type: integer
//
// responses:
//
//	200: RefResult
//	400:
//	500:

func RefsHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetRefs(ctx, r)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// swagger:route GET /v1/file/refs/recent/{allocation} GetRecentRefs
// Get recent references.
// Retrieve recent references added to an allocation, starting at a specific date, organized in a paginated table.
//
// parameters:
//
//	+name: allocation
//	   description: allocation ID
//	   required: true
//	   in: path
//	   type: string
//	+name: X-App-Client-ID
//     description: The ID/Wallet address of the client sending the request.
//     in: header
//     type: string
//     required: true
//	+name: X-App-Client-Key
// 	   description: The key of the client sending the request.
//     in: header
//     type: string
//     required: true
//	+name: ALLOCATION-ID
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
//  +name: limit
//    description: Number of records to show per page. If provided more than 100, it will be set to 100. Default is 20.
//    in: query
//    type: integer
//  +name: offset
//    description: Pagination offset. Default is 0.
//    in: query
//    type: string
//  +name: from-date
//    description: Timestamp to start listing from. Ignored if not provided.
//    in: query
//    type: integer
//
// responses:
//
//	200: RecentRefResult
//	400:
//	500:

func RecentRefsRequestHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.GetRecentlyAddedRefs(ctx, r)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// swagger:route POST /v1/file/rename/{allocation} PostRename
// Rename file.
// Rename a file in an allocation. Can only be run by the owner of the allocation.
// The allocation should permit rename for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.
//
// parameters:
//
//	+name: allocation
//	   description: the allocation ID
//	   required: true
//	   in: path
//	   type: string
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
//  +name: path
//     description: Path of the file to be renamed. Required only if `path_hash` is not provided.
//     in: query
//     type: string
//  +name: path_hash
//     description: Hash of the path of the file to be renamed. Required only if `path` is not provided.
//     in: query
//     type: string
//  +name: new_name
//     description: Name to be set to the file/directory.
//     in: query
//     type: string
//     required: true
//  +name: connection_id
//     description: Connection ID related to this process. Blobber uses the connection id to redeem rewards for storage and distinguish the operation. Connection should be using the create connection endpoint.
//     in: query
//     type: string
//     required: true
//
// responses:
//
//	200: UploadResult
//	400:
//	500:

func RenameHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.RenameObject(ctx, r)
	if err != nil {
		Logger.Error("renameHandler", zap.Error(err))
		return nil, err
	}
	return response, nil
}

// swagger:route POST /v1/file/copy/{allocation} PostCopy
// Copy a file.
// Copy a file in an allocation. Can only be run by the owner of the allocation.
// The allocation should permit copy for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.
//
// parameters:
//
//	+name: allocation
//	   description: the allocation ID
//	   required: true
//	   in: path
//	   type: string
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
//  +name: path
//     description: Path of the file to be copied. Required only if `path_hash` is not provided.
//     in: query
//     type: string
//  +name: path_hash
//     description: Hash of the path of the file to be copied. Required only if `path` is not provided.
//     in: query
//     type: string
//  +name: dest
//     description: Destination path of the file to be copied.
//     in: query
//     type: string
//     required: true
//  +name: connection_id
//     description: Connection ID related to this process. Blobber uses the connection id to redeem rewards for storage operations and distinguish the operation. Connection should be using the create connection endpoint.
//     in: query
//     type: string
//     required: true
//
// responses:
//
//	200: UploadResult
//	400:
//	500:

func CopyHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.CopyObject(ctx, r)
	if err != nil {
		Logger.Error("copyHandler", zap.Error(err))
		return nil, err
	}
	return response, nil
}

// swagger:route POST /v1/file/move/{allocation} PostMove
// Move a file.
// Mova a file from a path to another in an allocation. Can only be run by the owner of the allocation.
// The allocation should permit move for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.
//
// parameters:
//
//	+name: allocation
//	   description: the allocation ID
//	   required: true
//	   in: path
//	   type: string
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
//  +name: path
//     description: Path of the file to be moved. Required only if `path_hash` is not provided.
//     in: query
//     type: string
//  +name: path_hash
//     description: Hash of the path of the file to be moved. Required only if `path` is not provided.
//     in: query
//     type: string
//  +name: dest
//     description: Destination path of the file to be moved.
//     in: query
//     type: string
//     required: true
//  +name: connection_id
//     description: Connection ID related to this process. Blobber uses the connection id to redeem rewards for storage operations and distinguish the operation. Connection should be using the create connection endpoint.
//     in: query
//     type: string
//     required: true
//
// responses:
//
//	200: UploadResult
//	400:
//	500:

func MoveHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.MoveObject(ctx, r)
	if err != nil {
		Logger.Error("moveHandler", zap.Error(err))
		return nil, err
	}
	return response, nil
}

// swagger:route POST /v1/dir/{allocation} PostCreateDir
// Create a directory.
// Creates a directory in an allocation. Can only be run by the owner of the allocation.
//
// parameters:
//
//	+name: allocation
//	   description: the allocation ID
//	   required: true
//	   in: path
//	   type: string
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
//  +name: dir_path
//     description: Path of the directory to be created.
//     in: query
//     type: string
//     required: true
//
// responses:
//
//	200: UploadResult
//	400:
//	500:

func CreateDirHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.CreateDir(ctx, r)
	if err != nil {
		Logger.Error("createDirHandler", zap.Error(err))
		return nil, err
	}
	return response, nil
}

// swagger:route PUT /v1/file/upload/{allocation} PutUpdateFile
// Update/Replace a file.
// UpdateHandler is the handler to respond to update requests from clients. The allocation should permit update for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.
//
// parameters:
//
//	+name: allocation
//	   description: the allocation ID
//	   required: true
//	   in: path
//	   type: string
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
//	+name: connection_id
//	   description: ID of the connection related to this process. Check 2-PC documentation.
//	   required: true
//	   in: query
//	   type: string
//  +name: uploadMeta
//     description: Metadata of the file to be replaced with the current file. It should be a valid JSON object following the UploadFileChanger schema.
//     in: form
//     type: string
//     required: true
//  +name: uploadThumbnailFile
//    description: Thumbnail file to be replaced. It should be a valid image file.
//    in: form
//    type: file
//  +name: uploadFile
//    description: File to replace the existing one.
//    in: form
//    type: file
//    required: true
//
// responses:
//
//	200: UploadResult
//	400:
//	500:

// swagger:route DELETE /v1/file/upload/{allocation} DeleteFile
// Delete a file.
// DeleteHandler is the handler to respond to delete requests from clients. The allocation should permit delete for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.
//
// parameters:
//
//	+name: allocation
//	   description: the allocation ID
//	   required: true
//	   in: path
//	   type: string
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
//	+name: connection_id
//	   description: ID of the connection related to this process. Check 2-PC documentation.
//	   required: true
//	   in: query
//	   type: string
//  +name: path
//     description: Path of the file to be deleted.
//     in: query
//     type: string
//     required: true
//
// responses:
//
//	200: UploadResult
//	400:
//	500:

// swagger:route POST /v1/file/upload/{allocation} PostUploadFile
// Upload a file.
// uploadHandler is the handler to respond to upload requests from clients. The allocation should permit upload for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.
//
// parameters:
//
//		+name: allocation
//		   description: the allocation ID
//		   required: true
//		   in: path
//		   type: string
//		 +name: X-App-Client-ID
//	    description: The ID/Wallet address of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		 +name: X-App-Client-Key
//		   description: The key of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		 +name: ALLOCATION-ID
//		   description: The ID of the allocation in question.
//	    in: header
//	    type: string
//	    required: true
//	 +name: X-App-Client-Signature
//	    description: Digital signature of the client used to verify the request if the X-Version is not "v2"
//	    in: header
//	    type: string
//	 +name: X-App-Client-Signature-V2
//	    description: Digital signature of the client used to verify the request if the X-Version is "v2"
//	    in: header
//	    type: string
//		+name: connection_id
//		   description: ID of the connection related to this process. Check 2-PC documentation.
//		   required: true
//		   in: query
//		   type: string
//	 +name: uploadMeta
//	    description: Metadata of the file to be uploaded. It should be a valid JSON object following the UploadFileChanger schema.
//	    in: form
//	    type: string
//	    required: true
//	 +name: uploadThumbnailFile
//	   description: Thumbnail file to be uploaded. It should be a valid image file.
//	   in: form
//	   type: file
//	 +name: uploadFile
//	   description: File to be uploaded.
//	   in: form
//	   type: file
//	   required: true
//
// responses:
//
//	200: UploadResult
//	400:
//	500:
func uploadHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)
	response, err := storageHandler.WriteFile(ctx, r)
	if err != nil {
		Logger.Error("writeFileHandler", zap.Error(err))
		return nil, err
	}
	return response, nil
}

func RedeemHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return redeemHandler(ctx, r)
}

func writeResponse(w http.ResponseWriter, resp []byte) {
	_, err := w.Write(resp)

	if err != nil {
		Logger.Error("Error sending StatsHandler response", zap.Error(err))
	}
}

func GetBlobberInfo(ctx context.Context, r *http.Request) (interface{}, error) {
	blobberInfo := GetBlobberInfoJson()
	return blobberInfo, nil
}

// todo wrap with connection
func StatsHandler(w http.ResponseWriter, r *http.Request) {
	isJSON := r.Header.Get("Accept") == "application/json"
	if isJSON {
		var (
			blobberStats any
			err          error
		)
		blobberInfo := GetBlobberInfoJson()
		err = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
			blobberStats, err = stats.StatsJSONHandler(ctx, r)
			return err
		})

		if err != nil {
			Logger.Error("Error getting blobber JSON stats", zap.Error(err))

			w.WriteHeader(http.StatusInternalServerError)
			writeResponse(w, []byte(err.Error()))
			return
		}

		blobberInfo.Stats = blobberStats

		statsJson, err := json.Marshal(blobberInfo)

		if err != nil {
			Logger.Error("Error marshaling JSON stats", zap.Error(err))

			w.WriteHeader(http.StatusInternalServerError)
			writeResponse(w, []byte(err.Error()))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeResponse(w, statsJson)

		return
	}

	HTMLHeader(w, "Blobber Diagnostics")
	PrintCSS(w)
	HomepageHandler(w, r)

	if getBlobberHealthCheckError() != nil {
		r.Header.Set(stats.HealthDataKey.String(), "✗")
	} else {
		r.Header.Set(stats.HealthDataKey.String(), "✔")
	}

	stats.StatsHandler(w, r)
	HTMLFooter(w)
}

//nolint:gosimple // need more time to verify
func HandleShutdown(ctx context.Context) {

	go func() {
		<-ctx.Done()
		Logger.Info("Shutting down server")
		datastore.GetStore().Close()
		store := datastore.GetBlockStore()
		if store != nil {
			store.Close()
		}
	}()
}

func DumpGoRoutines(ctx context.Context, r *http.Request) (interface{}, error) {

	_ = pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
	return "success", nil
}

func GetConfig(ctx context.Context, r *http.Request) (interface{}, error) {

	return config.Configuration, nil
}

func GetObjectLimit(ctx context.Context, r *http.Request) (interface{}, error) {
	var objLimit struct {
		MaxAllocationDirFiles int
		MaxObjectsInDir       int
	}
	objLimit.MaxAllocationDirFiles = config.Configuration.MaxAllocationDirFiles
	objLimit.MaxObjectsInDir = config.Configuration.MaxObjectsInDir
	return objLimit, nil
}

func GetLogs(ctx context.Context, r *http.Request) (interface{}, error) {
	return transaction.Last50Transactions, nil
}

func CleanupDiskHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	err := CleanupDiskFiles(ctx)
	return "cleanup", err
}

// swagger:route DELETE /v1/marketplace/shareinfo/{allocation} DeleteShare
// Revokes access to a shared file.
// Handle revoke share requests from clients.
//
// parameters:
//   +name: allocation
//     description: TxHash of the allocation in question.
//     in: path
//     required: true
//     type: string
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
//     description: Digital signature of the client used to verify the request.
//     in: header
//     type: string
//     required: true
//  +name: X-App-Client-Signature-V2
//     description: Digital signature of the client used to verify the request. Overrides X-App-Client-Signature if provided.
//     in: header
//     type: string
// +name: path
//     description: Path of the file to be shared.
//     in: query
//     type: string
//     required: true
// +name: refereeClientID
//     description: The ID of the client to revoke access to the file (in case of private sharing).
//     in: query
//     type: string
//
// responses:
//
//	200:
//  400:

func RevokeShare(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)

	allocationID := ctx.Value(constants.ContextKeyAllocationID).(string)
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := storageHandler.verifyAllocation(ctx, allocationID, allocationTx, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation ID passed."+err.Error())
	}

	sign := r.Header.Get(common.ClientSignatureHeader)
	signV2 := r.Header.Get(common.ClientSignatureHeaderV2)

	valid, err := verifySignatureFromRequest(allocationTx, sign, signV2, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	path, _ := common.GetField(r, "path")
	if path == "" {
		return nil, common.NewError("invalid_parameters", "Invalid file path")
	}
	refereeClientID, _ := common.GetField(r, "refereeClientID")
	filePathHash := fileref.GetReferenceLookup(allocationID, path)
	_, err = reference.GetLimitedRefFieldsByLookupHash(ctx, allocationID, filePathHash, []string{"id", "type"})
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID != allocationObj.OwnerID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	err = reference.DeleteShareInfo(ctx, &reference.ShareInfo{
		ClientID:     refereeClientID,
		FilePathHash: filePathHash,
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		resp := map[string]interface{}{
			"status":  http.StatusNotFound,
			"message": "Path not found",
		}
		return resp, nil
	}

	if err != nil {
		return nil, err
	}

	resp := map[string]interface{}{
		"status":  http.StatusNoContent,
		"message": "Path successfully removed from allocation",
	}
	return resp, nil
}

// swagger:route POST /v1/marketplace/shareinfo/{allocation} PostShareInfo
// Share a file.
// Handle share file requests from clients. Returns generic mapping.
//
// parameters:
//   +name: allocation
//     description: TxHash of the allocation in question.
//     in: path
//     required: true
//     type: string
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
//     description: Digital signature of the client used to verify the request.
//     in: header
//     type: string
//     required: true
//  +name: X-App-Client-Signature-V2
//     description: Digital signature of the client used to verify the request. Overrides X-App-Client-Signature if provided.
//     in: header
//     type: string
//  +name: encryption_public_key
//     description: Public key of the referee client in case of private sharing. Used for proxy re-encryption.
//     in: form
//     type: string
//  +name: available_after
//     description: Time after which the file will be accessible for sharing.
//     in: form
//     type: string
//  +name: auth_ticket
//     description: Body of the auth ticket used to verify the file access. Follows the structure of [`AuthTicket`](#auth-ticket)
//     in: form
//     type: string
//     required: true
//
// responses:
//
//	200:
//	400:

func InsertShare(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)

	var (
		allocationTx = ctx.Value(constants.ContextKeyAllocation).(string)
		allocationID = ctx.Value(constants.ContextKeyAllocationID).(string)
		clientID     = ctx.Value(constants.ContextKeyClient).(string)
	)

	allocationObj, err := storageHandler.verifyAllocation(ctx, allocationID, allocationTx, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation ID passed."+err.Error())
	}

	sign := r.Header.Get(common.ClientSignatureHeader)
	signV2 := r.Header.Get(common.ClientSignatureHeaderV2)

	valid, err := verifySignatureFromRequest(allocationTx, sign, signV2, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	if clientID != allocationObj.OwnerID {
		return nil, common.NewError("invalid_client", "Client has no access to share file")
	}

	encryptionPublicKey := r.FormValue("encryption_public_key")
	authTicketString := r.FormValue("auth_ticket")
	availableAfter := r.FormValue("available_after")
	authTicket := &readmarker.AuthTicket{}

	err = json.Unmarshal([]byte(authTicketString), &authTicket)
	if err != nil {
		return false, common.NewError("invalid_parameters", "Error parsing the auth ticket for download."+err.Error())
	}
	fileRef, err := reference.GetLimitedRefFieldsByLookupHash(ctx, allocationID, authTicket.FilePathHash, []string{"id", "path", "lookup_hash", "type", "name"})
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	authToken, err := storageHandler.verifyAuthTicket(ctx, authTicketString, allocationObj, fileRef, authTicket.ClientID, false)
	if authToken == nil {
		return nil, common.NewError("auth_ticket_verification_failed", "Could not verify the auth ticket. "+err.Error())
	}

	if err != nil {
		return nil, err
	}

	availableAt := common.Now()

	if len(availableAfter) > 0 {
		a, err := strconv.ParseInt(availableAfter, 10, 64)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid available_after: "+err.Error())
		}
		availableAt = common.Timestamp(a)
	}

	shareInfo := reference.ShareInfo{
		OwnerID:                   authTicket.OwnerID,
		ClientID:                  authTicket.ClientID,
		FilePathHash:              authTicket.FilePathHash,
		ReEncryptionKey:           authTicket.ReEncryptionKey,
		ClientEncryptionPublicKey: encryptionPublicKey,
		ExpiryAt:                  common.ToTime(authTicket.Expiration).UTC(),
		AvailableAt:               common.ToTime(availableAt).UTC(),
	}

	existingShare, _ := reference.GetShareInfo(ctx, authTicket.ClientID, authTicket.FilePathHash)

	if existingShare != nil && len(existingShare.OwnerID) > 0 {
		err = reference.UpdateShareInfo(ctx, &shareInfo)
	} else {
		err = reference.AddShareInfo(ctx, &shareInfo)
	}

	if err != nil {
		Logger.Info(err.Error())
		return nil, common.NewError("share_info_insert", "Unable to save share info")
	}

	return map[string]interface{}{"message": "Share info added successfully"}, nil
}

// swagger:route GET /v1/marketplace/shareinfo/{allocation} GetListShareInfo
// List shared files.
// Retrieve shared files in an allocation by its owner.
//
// parameters:
//
//		+name: allocation
//		   description: the allocation ID
//		   required: true
//		   in: path
//		   type: string
//		 +name: X-App-Client-ID
//	    description: The ID/Wallet address of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		 +name: X-App-Client-Key
//		   description: The key of the client sending the request.
//	    in: header
//	    type: string
//	    required: true
//		 +name: ALLOCATION-ID
//		   description: The ID of the allocation in question.
//	    in: header
//	    type: string
//	    required: true
//	 +name: X-App-Client-Signature
//	    description: Digital signature of the client used to verify the request if the X-Version is not "v2"
//	    in: header
//	    type: string
//	 +name: X-App-Client-Signature-V2
//	    description: Digital signature of the client used to verify the request if the X-Version is "v2"
//	    in: header
//	    type: string
//	  +name: offset
//	    in: query
//	    type: integer
//	    required: false
//	    description: Pagination offset, start of the page to retrieve. Default is 0.
//	  +name: limit
//	    in: query
//	    type: integer
//	    required: false
//	    description: Pagination limit, number of entries in the page to retrieve. Default is 20.
//	  +name: sort
//	    in: query
//	    type: string
//	    required: false
//	    description: Direction of sorting based on challenge closure time, either "asc" or "desc". Default is "asc"
//
// responses:
//
//	200: []ShareInfo
//	400:
//	500:
func ListShare(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)

	var (
		allocationTx = ctx.Value(constants.ContextKeyAllocation).(string)
		allocationID = ctx.Value(constants.ContextKeyAllocationID).(string)
		clientID     = ctx.Value(constants.ContextKeyClient).(string)
	)

	limit, err := common.GetOffsetLimitOrderParam(r.URL.Query())
	if err != nil {
		return nil, err
	}

	allocationObj, err := storageHandler.verifyAllocation(ctx, allocationID, allocationTx, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	sign := r.Header.Get(common.ClientSignatureHeader)
	signV2 := r.Header.Get(common.ClientSignatureHeaderV2)

	valid, err := verifySignatureFromRequest(allocationTx, sign, signV2, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	if clientID != allocationObj.OwnerID {
		return nil, common.NewError("invalid_client", "Client has no access to share file")
	}

	shares, err := reference.ListShareInfoClientID(ctx, clientID, limit)
	if err != nil {
		Logger.Error("failed_to_list_share", zap.Error(err))
		return nil, common.NewError("failed_to_list_share", "failed to list file share")
	}

	// get the files shared in that allocation
	return shares, nil
}

// PrintCSS - print the common css elements
func PrintCSS(w http.ResponseWriter) {
	fmt.Fprintf(w, "<style>\n")
	fmt.Fprintf(w, ".number { text-align: right; }\n")
	fmt.Fprintf(w, ".fixed-text { overflow:hidden;white-space: nowrap;word-break: break-all;word-wrap: break-word; text-overflow: ellipsis; }\n")
	fmt.Fprintf(w, ".menu li { list-style-type: none; }\n")
	fmt.Fprintf(w, ".page-item { display:inline-block; padding:10px;}\n")
	fmt.Fprintf(w, ".pagination {float: right; margin-right: 24px;}\n")
	fmt.Fprintf(w, "table, td, th { border: 1px solid black;  border-collapse: collapse;}\n")
	fmt.Fprintf(w, ".tname { width: 70%%}\n")
	fmt.Fprintf(w, "tr.header { background-color: #E0E0E0;  }\n")
	fmt.Fprintf(w, ".inactive { background-color: #F44336; }\n")
	fmt.Fprintf(w, ".warning { background-color: #FFEB3B; }\n")
	fmt.Fprintf(w, ".optimal { color: #1B5E20; }\n")
	fmt.Fprintf(w, ".slow { font-style: italic; }\n")
	fmt.Fprintf(w, ".bold {font-weight:bold;}")
	fmt.Fprintf(w, "tr.green td {background-color:light-green;}")
	fmt.Fprintf(w, "tr.grey td {background-color:light-grey;}")
	fmt.Fprintf(w, "</style>")
}

func HTMLHeader(w http.ResponseWriter, title string) {
	fmt.Fprintf(w, "<!DOCTYPE html><html><head><title>%s</title></head><body>", title)
}
func HTMLFooter(w http.ResponseWriter) {
	fmt.Fprintf(w, "</body></html>")
}
