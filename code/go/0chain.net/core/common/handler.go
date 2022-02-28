package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const (
	// AppErrorHeader - a http response header to send an application error code.
	AppErrorHeader = "X-App-Error-Code"

	ClientHeader    = "X-App-Client-ID"
	ClientKeyHeader = "X-App-Client-Key"
	TimestampHeader = "X-App-Timestamp"

	// ClientSignatureHeader represents http request header contains signature.
	ClientSignatureHeader = "X-App-Client-Signature"
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
	if err != nil {
		data := make(map[string]interface{}, 2)
		data["error"] = err.Error()
		if cerr, ok := err.(*Error); ok {
			data["code"] = cerr.Code
		}
		buf := bytes.NewBuffer(nil)
		json.NewEncoder(buf).Encode(data) //nolint:errcheck // checked in previous step
		http.Error(w, buf.String(), 400)
	} else if data != nil {
		json.NewEncoder(w).Encode(data) //nolint:errcheck // checked in previous step
	}
}

var domainRE = regexp.MustCompile(`^(?:https?:\/\/)?(?:[^@\/\n]+@)?(?:www\.)?([^:\/\n]+)`) //nolint:unused,deadcode,varcheck // might be used later?

func ToByteStream(handler JSONResponderF) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		data, err := handler(ctx, r)
		if err != nil {
			statusCode := 400
			if cerr, ok := err.(*Error); ok {
				w.Header().Set(AppErrorHeader, cerr.Code)
				if cerr.StatusCode != 0 {
					statusCode = cerr.StatusCode
				}
			}
			if data != nil {
				responseString, _ := json.Marshal(data)
				http.Error(w, string(responseString), statusCode)
			} else {
				http.Error(w, err.Error(), statusCode)
			}
		} else if data != nil {
			rawdata, ok := data.([]byte)
			if ok {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Write(rawdata) //nolint:errcheck
			} else {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(data) //nolint:errcheck
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
