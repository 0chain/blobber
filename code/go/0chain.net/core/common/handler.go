package common

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/pierrec/lz4/v4"
)

const (
	// AppErrorHeader - a http response header to send an application error code.
	AppErrorHeader = "X-App-Error-Code"

	ClientHeader    = "X-App-Client-ID"
	ClientKeyHeader = "X-App-Client-Key"
	TimestampHeader = "X-App-Timestamp"

	// ClientSignatureHeader represents http request header contains signature.
	ClientSignatureHeader   = "X-App-Client-Signature"
	ClientSignatureHeaderV2 = "X-App-Client-Signature-V2"

	AllocationIdHeader = "ALLOCATION-ID"
)

/*ReqRespHandlerf - a type for the default handler signature */
type ReqRespHandlerf func(w http.ResponseWriter, r *http.Request)

/*JSONResponderF - a handler that takes standard request (non-json) and responds with a json response
* Useful for POST opertaion where the input is posted as json with
*    Content-type: application/json
* header
 */
type JSONResponderF func(ctx context.Context, r *http.Request) (interface{}, error)

/*JSONReqResponderF - a handler that takes a JSON request and responds with a json response
* Useful for GET operation where the input is coming via url parameters
 */
type JSONReqResponderF func(ctx context.Context, json map[string]interface{}) (interface{}, error)

/*Respond - respond either data or error as a response */
func Respond(w http.ResponseWriter, data interface{}, err error) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err != nil {
		data := make(map[string]interface{}, 2)
		data["error"] = err.Error()
		if cerr, ok := err.(*Error); ok {
			data["code"] = cerr.Code
		}
		buf := bytes.NewBuffer(nil)
		json.NewEncoder(buf).Encode(data) //nolint:errcheck // checked in previous step
		w.WriteHeader(400)
		fmt.Fprintln(w, buf.String())
	} else if data != nil {
		json.NewEncoder(w).Encode(data) //nolint:errcheck // checked in previous step
	}
}

func RespondGzip(w http.ResponseWriter, data any, err error) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err != nil {
		data := make(map[string]interface{}, 2)
		data["error"] = err.Error()
		if cerr, ok := err.(*Error); ok {
			data["code"] = cerr.Code
		}
		buf := bytes.NewBuffer(nil)
		json.NewEncoder(buf).Encode(data) //nolint:errcheck // checked in previous step
		w.WriteHeader(400)
		fmt.Fprintln(w, buf.String())
	} else if data != nil {
		w.Header().Set("Content-Encoding", "gzip")
		gw := gzip.NewWriter(w)
		defer gw.Close()
		json.NewEncoder(gw).Encode(data) //nolint:errcheck // checked in previous step
	}
}

func RespondLz4(w http.ResponseWriter, data any, err error) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err != nil {
		data := make(map[string]interface{}, 2)
		data["error"] = err.Error()
		if cerr, ok := err.(*Error); ok {
			data["code"] = cerr.Code
		}
		buf := bytes.NewBuffer(nil)
		json.NewEncoder(buf).Encode(data) //nolint:errcheck // checked in previous step
		w.WriteHeader(400)
		fmt.Fprintln(w, buf.String())
	} else if data != nil {
		w.Header().Set("Content-Encoding", "lz4")
		lw := lz4.NewWriter(w)
		defer lw.Close()
		json.NewEncoder(lw).Encode(data) //nolint:errcheck // checked in previous step
	}
}

var domainRE = regexp.MustCompile(`^(?:https?:\/\/)?(?:[^@\/\n]+@)?(?:www\.)?([^:\/\n]+)`) //nolint:unused,deadcode,varcheck // might be used later?

func ToByteStream(handler JSONResponderF) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		data, err := handler(ctx, r)
		if err != nil {
			if cerr, ok := err.(*Error); ok {
				w.Header().Set(AppErrorHeader, cerr.Code)
			}
			if data != nil {
				responseString, _ := json.Marshal(data)
				http.Error(w, string(responseString), 400)
			} else {
				http.Error(w, err.Error(), 400)
			}
		} else if data != nil {
			rawdata, ok := data.([]byte)
			if ok {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(rawdata)))
				w.Write(rawdata) //nolint:errcheck
			} else {
				w.Header().Set("Content-Type", "application/json")
				byteData, err := json.Marshal(data)
				if err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(byteData)))
				w.Write(byteData) //nolint:errcheck
			}
		}
	}
}

func SetupCORSResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Accept-Encoding")
}

/*ToJSONResponse - An adapter that takes a handler of the form
* func AHandler(r *http.Request) (interface{}, error)
* which takes a request object, processes and returns an object or an error
* and converts into a standard request/response handler
 */
func ToJSONResponse(handler JSONResponderF) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
		if r.Method == "OPTIONS" {
			SetupCORSResponse(w, r)
			return
		}
		ctx := r.Context()
		data, err := handler(ctx, r)
		Respond(w, data, err)
	}
}

/*ToGzipJSONResponse - An adapter that takes a handler of the form
* func AHandler(r *http.Request) (interface{}, error)
* which takes a request object, processes and returns an object or an error
* and converts into a standard request/response handler
 */
func ToGzipJSONResponse(handler JSONResponderF) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
		if r.Method == "OPTIONS" {
			SetupCORSResponse(w, r)
			return
		}
		ctx := r.Context()
		data, err := handler(ctx, r)
		RespondGzip(w, data, err)
	}
}

/*ToJSONReqResponse - An adapter that takes a handler of the form
* func AHandler(json map[string]interface{}) (interface{}, error)
* which takes a parsed json map from the request, processes and returns an object or an error
* and converts into a standard request/response handler
 */
func ToJSONReqResponse(handler JSONReqResponderF) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-type")
		if !strings.HasPrefix(contentType, "application/json") {
			http.Error(w, "Header Content-type=application/json not found", 400)
			return
		}
		decoder := json.NewDecoder(r.Body)
		var jsonData map[string]interface{}
		err := decoder.Decode(&jsonData)
		if err != nil {
			http.Error(w, "Error decoding json", 500)
			return
		}
		ctx := r.Context()
		data, err := handler(ctx, jsonData)
		Respond(w, data, err)
	}
}

/*JSONString - given a json map and a field return the string typed value
* required indicates whether to throw an error if the field is not found
 */
func JSONString(json map[string]interface{}, field string, required bool) (string, error) {
	val, ok := json[field]
	if !ok {
		if required {
			return "", fmt.Errorf("input %v is required", field)
		}
		return "", nil
	}
	switch sval := val.(type) {
	case string:
		return sval, nil
	default:
		return fmt.Sprintf("%v", sval), nil
	}
}

type StatusCodeResponderF func(ctx context.Context, r *http.Request) (interface{}, int, error)

func ToStatusCode(handler StatusCodeResponderF) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
		if r.Method == "OPTIONS" {
			SetupCORSResponse(w, r)
			return
		}

		ctx := r.Context()

		data, statusCode, err := handler(ctx, r)

		if err != nil {
			if statusCode == 0 {
				statusCode = http.StatusBadRequest
			}

			w.WriteHeader(statusCode)
			w.Header().Set("Content-Type", "application/json")

			if data != nil {
				json.NewEncoder(w).Encode(data) //nolint:errcheck
			} else {
				//nolint:errcheck
				json.NewEncoder(w).Encode(map[string]string{
					"error": err.Error(),
				})
			}

			return
		}

		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		w.WriteHeader(statusCode)

		if data != nil {

			rawdata, ok := data.([]byte)
			if ok {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(rawdata)))
				w.Write(rawdata) //nolint:errcheck
			} else {
				w.Header().Set("Content-Type", "application/json")
				jsonData, _ := json.Marshal(data)
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(jsonData)))
				w.Write(jsonData) //nolint:errcheck
			}
		}
	}
}

/*JSONResponderOrNotF - a handler that takes standard request (non-json) and responds with a json response
* Useful for POST opertaion where the input is posted as json with
*    Content-type: application/json
* header
* For test purposes it is useful to not respond
 */
type JSONResponderOrNotF func(ctx context.Context, r *http.Request) (interface{}, error, bool)

/* ToJSONOrNotResponse - An adapter that takes a handler of the form
* func AHandler(r *http.Request) (interface{}, error)
* which takes a request object, processes and returns an object or an error
* and converts into a standard request/response handler or simply does not respond
* for test purposes
 */
func ToJSONOrNotResponse(handler JSONResponderOrNotF) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
		if r.Method == "OPTIONS" {
			SetupCORSResponse(w, r)
			return
		}
		ctx := r.Context()
		data, err, shouldRespond := handler(ctx, r)

		if shouldRespond {
			Respond(w, data, err)
		}
	}
}
