//	0chain Blobber API:
//	 version: 0.0.1
//	 title: 0chain Blobber API
//	Schemes: http, https
//	BasePath: /
//	Produces:
//	  - application/json
//
// securityDefinitions:
//
//	apiKey:
//	  type: apiKey
//	  in: header
//	  name: authorization
//
// swagger:meta
package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/0chain/gosdk/core/zcncrypto"
	"net/http"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

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

/*setupHandlers sets up the necessary API end points */
func setupHandlers(r *mux.Router) {
	ConfigRateLimits()
	r.Use(UseRecovery, UseCors)

	//object operations
	r.HandleFunc("/v1/connection/create/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(CreateConnectionHandler)))).
		Methods(http.MethodPost)

	r.HandleFunc("/v1/connection/redeem/{allocation}",
		RateLimitByGeneralRL(common.ToByteStream(WithConnection(RedeemHandler)))).
		Methods(http.MethodPost, http.MethodOptions)

	r.HandleFunc("/v1/file/rename/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(RenameHandler)))).
		Methods(http.MethodPost, http.MethodOptions)

	r.HandleFunc("/v1/file/copy/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(CopyHandler)))).
		Methods(http.MethodPost, http.MethodOptions)

	r.HandleFunc("/v1/file/move/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(MoveHandler)))).
		Methods(http.MethodPost, http.MethodOptions)

	r.HandleFunc("/v1/dir/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(CreateDirHandler)))).
		Methods(http.MethodPost, http.MethodDelete, http.MethodOptions)

	r.HandleFunc("/v1/connection/commit/{allocation}",
		RateLimitByCommmitRL(common.ToStatusCode(WithStatusConnectionForWM(CommitHandler))))

	r.HandleFunc("/v1/connection/rollback/{allocation}",
		RateLimitByCommmitRL(common.ToStatusCode(WithStatusConnectionForWM(RollbackHandler))))

	//object info related apis
	r.HandleFunc("/allocation",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(AllocationHandler))))

	r.HandleFunc("/v1/file/meta/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithReadOnlyConnection(FileMetaHandler)))) // TODO: add swagger

	r.HandleFunc("/v1/file/stats/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithReadOnlyConnection(FileStatsHandler)))) // TODO: add swagger

	r.HandleFunc("/v1/file/referencepath/{allocation}",
		RateLimitByObjectRL(common.ToJSONResponse(WithReadOnlyConnection(ReferencePathHandler)))) // TODO: add handler

	r.HandleFunc("/v1/file/latestwritemarker/{allocation}",
		RateLimitByObjectRL(common.ToJSONResponse(WithReadOnlyConnection(WriteMarkerHandler))))

	r.HandleFunc("/v1/file/objecttree/{allocation}",
		RateLimitByObjectRL(common.ToStatusCode(WithStatusReadOnlyConnection(ObjectTreeHandler)))).
		Methods(http.MethodGet, http.MethodOptions)

	r.HandleFunc("/v1/file/refs/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithReadOnlyConnection(RefsHandler)))).
		Methods(http.MethodGet, http.MethodOptions)

	r.HandleFunc("/v1/file/refs/recent/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithReadOnlyConnection(RecentRefsRequestHandler)))).
		Methods(http.MethodGet, http.MethodOptions)

	// admin related
	// Allowing admin api for debugging purpose only. Later on commented out line should be
	// uncommented and line below it should be deleted

	r.HandleFunc("/_debug", common.AuthenticateAdmin(common.ToJSONResponse(DumpGoRoutines)))
	// r.HandleFunc("/_debug", RateLimitByCommmitRL(common.ToJSONResponse(DumpGoRoutines)))
	r.HandleFunc("/_config", common.AuthenticateAdmin(common.ToJSONResponse(GetConfig)))
	// r.HandleFunc("/_config", RateLimitByCommmitRL(common.ToJSONResponse(GetConfig)))
	// r.HandleFunc("/_stats", common.AuthenticateAdmin(StatsHandler))
	r.HandleFunc("/_stats", RateLimitByCommmitRL(StatsHandler))

	r.HandleFunc("/_logs", RateLimitByCommmitRL(common.ToJSONResponse(GetLogs)))

	// r.HandleFunc("/_cleanupdisk", common.AuthenticateAdmin(common.ToJSONResponse(WithReadOnlyConnection(CleanupDiskHandler))))
	// r.HandleFunc("/_cleanupdisk", RateLimitByCommmitRL(common.ToJSONResponse(WithReadOnlyConnection(CleanupDiskHandler))))
	r.HandleFunc("/challengetimings", common.AuthenticateAdmin(common.ToJSONResponse(GetChallengeTimings)))
	// r.HandleFunc("/challengetimings", RateLimitByCommmitRL(common.ToJSONResponse(GetChallengeTimings)))
	r.HandleFunc("/challenge-timings-by-challengeId", RateLimitByCommmitRL(common.ToJSONResponse(GetChallengeTiming)))

	// Generate auth ticket
	r.HandleFunc("/v1/auth/generate", common.ToJSONResponse(With0boxAuth(GenerateAuthTicket))).Methods(http.MethodGet, http.MethodOptions)

	//marketplace related
	r.HandleFunc("/v1/marketplace/shareinfo/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(InsertShare)))).
		Methods(http.MethodOptions, http.MethodPost)

	r.HandleFunc("/v1/marketplace/shareinfo/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(RevokeShare)))).
		Methods(http.MethodOptions, http.MethodDelete)

	// list files shared in this allocation
	r.HandleFunc("/v1/marketplace/shareinfo/{allocation}",
		RateLimitByGeneralRL(common.ToJSONResponse(WithConnection(ListShare)))).
		Methods(http.MethodOptions, http.MethodGet)

	// lightweight http handler without heavy postgres transaction to improve performance

	r.HandleFunc("/v1/writemarker/lock/{allocation}",
		RateLimitByGeneralRL(WithTxHandler(LockWriteMarker))).
		Methods(http.MethodPost, http.MethodOptions)

	r.HandleFunc("/v1/writemarker/lock/{allocation}/{connection}",
		RateLimitByGeneralRL(WithTxHandler(UnlockWriteMarker))).
		Methods(http.MethodDelete, http.MethodOptions)

	r.HandleFunc("/v1/hashnode/root/{allocation}",
		RateLimitByObjectRL(WithTxHandler(LoadRootHashnode))).
		Methods(http.MethodGet, http.MethodOptions)

	r.HandleFunc("/v1/playlist/latest/{allocation}",
		RateLimitByGeneralRL(WithTxHandler(LoadPlaylist))).
		Methods(http.MethodGet, http.MethodOptions)

	r.HandleFunc("/v1/playlist/file/{allocation}",
		RateLimitByGeneralRL(WithTxHandler(LoadPlaylistFile))).
		Methods(http.MethodGet, http.MethodOptions)
}

func WithReadOnlyConnection(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = GetMetaDataStore().CreateTransaction(ctx)
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

func With0boxAuth(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		var (
			resp interface{}
			err  error
		)
		err = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
			zboxAuth := viper.GetString("0box.public_key")
			logging.Logger.Info("Jayash With0boxAuth", zap.Any("0box-public-key", common.PublicKey0box), zap.Any("zbox-public-key", zboxAuth))

			signature := r.Header.Get("Zbox-Signature")
			logging.Logger.Info("Jayash With0boxAuth", zap.Any("signature", signature))
			if signature == "" {
				return common.NewError("invalid_parameters", "Invalid signature")
			}

			signatureScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
			if err := signatureScheme.SetPublicKey(""); err != nil {
				return err
			}

			success, err := signatureScheme.Verify(signature, hex.EncodeToString([]byte(common.PublicKey0box)))
			logging.Logger.Info("Jayash With0boxAuth", zap.Any("success", success), zap.Any("", err))
			if err != nil || !success {
				return common.NewError("invalid_signature", "Invalid signature")
			}

			resp, err = handler(ctx, r)
			logging.Logger.Info("Jayash With0boxAuth", zap.Any("resp", resp), zap.Error(err))

			return err
		})
		return resp, err
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

// swagger:route GET /allocation allocation
// get allocation details
//
// parameters:
//
//	+name: id
//	 description: allocation ID
//	 required: true
//	 in: query
//	 type: string
//
// responses:
//
//	200: CommitResult
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

// swagger:route POST /v1/file/commitmetatxn/{allocation} commitmetatxn
// CommitHandler is the handler to respond to upload requests from clients
//
// parameters:
//
//	+name: auth_token
//	 description: auth token
//	 required: true
//	 in: body
//	 type: string
//
//	+name: txn_id
//	 description: transaction id
//	 required: true
//	 in: body
//	 type: string
//
// responses:
//
//	200:
//	400:
//	500:

// TODO: add swagger
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

// swagger:route GET /v1/connection/create/{allocation} connectionHandler
// connectionHandler is the handler to respond to create connection requests from clients
//
// parameters:
//
//	+name: allocation
//	 description: the allocation ID
//	 required: true
//	 in: path
//	 type: string
//
// responses:
//
//	200:
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

// swagger:route GET /v1/connection/commit/{allocation} commithandler
// CommitHandler is the handler to respond to upload requests from clients
//
// parameters:
//
//	+name: allocation
//	 description: the allocation ID
//	 required: true
//	 in: path
//	 type: string
//
// responses:
//
//	200: CommitResult
//	400:
//	500:

func CommitHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	return commitHandler(ctx, r)
}

func RollbackHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	return rollbackHandler(ctx, r)
}

func ReferencePathHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx, canceler := context.WithTimeout(ctx, time.Second*10)
	defer canceler()

	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetReferencePath(ctx, r)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func WriteMarkerHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	ctx = setupHandlerContext(ctx, r)

	response, err := storageHandler.GetLatestWriteMarker(ctx, r)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// swagger:route GET /v1/file/objecttree/{allocation} referencepath
// get object tree reference path
//
// parameters:
//
//	+name: allocation
//	 description: allocation ID
//	 required: true
//	 in: path
//	 type: string
//
// responses:
//
//	200: ReferencePathResult
//	400:
//	500:

func ObjectTreeHandler(ctx context.Context, r *http.Request) (interface{}, int, error) {
	return objectTreeHandler(ctx, r)
}

// swagger:route GET /v1/file/refs/{allocation} refshandler
// get object tree reference path
//
// parameters:
//
//	+name: allocation
//	 description: allocation ID
//	 required: true
//	 in: path
//	 type: string
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

// swagger:route GET /v1/file/refs/recent/{allocation} recentalloc
// get recent allocation
//
// parameters:
//
//	+name: allocation
//	 description: allocation ID
//	 required: true
//	 in: path
//	 type: string
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

// swagger:route GET /v1/file/rename/{allocation} renameallocation
// rename an allocation
//
// parameters:
//
//	+name: allocation
//	 description: the allocation ID
//	 required: true
//	 in: path
//	 type: string
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

// swagger:route GET /v1/file/copy/{allocation} copyallocation
// copy an allocation
//
// parameters:
//
//	+name: allocation
//	 description: the allocation ID
//	 required: true
//	 in: path
//	 type: string
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

// swagger:route GET /v1/file/move/{allocation} moveallocation
// move an allocation
//
// parameters:
//
//	+name: allocation
//	 description: the allocation ID
//	 required: true
//	 in: path
//	 type: string
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

// swagger:route GET /v1/dir/{allocation} createdirhandler
// CreateDirHandler is the handler to respond to create dir for allocation
//
// parameters:
//
//	+name: allocation
//	 description: the allocation ID
//	 required: true
//	 in: path
//	 type: string
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

/*uploadHandler is the handler to respond to upload requests fro clients*/
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
		select {
		case <-ctx.Done():
			Logger.Info("Shutting down server")
			datastore.GetStore().Close()
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

func GetLogs(ctx context.Context, r *http.Request) (interface{}, error) {
	return transaction.Last50Transactions, nil
}

func CleanupDiskHandler(ctx context.Context, r *http.Request) (interface{}, error) {

	err := CleanupDiskFiles(ctx)
	return "cleanup", err
}

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

// ListShare a list of files that clientID has shared
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
