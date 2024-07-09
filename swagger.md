


# Züs Blobber API.
Documentation of the blobber API.
  

## Informations

### Version

1.12.0

## Content negotiation

### URI Schemes
  * https

### Consumes
  * application/json
  * multipart/form-data
  * application/x-www-form-urlencoded

### Produces
  * application/json

## All endpoints

###  operations

| Method  | URI     | Name   | Summary |
|---------|---------|--------|---------|
| DELETE | /v1/file/upload/{allocation} | [delete file](#delete-file) | Delete a file. |
| DELETE | /v1/writemarker/lock/{allocation}/{connection} | [delete lock write marker](#delete-lock-write-marker) | Unlock a write marker. |
| DELETE | /v1/marketplace/shareinfo/{allocation} | [delete share](#delete-share) | Revokes access to a shared file. |
| GET | /allocation | [get allocation](#get-allocation) | Get allocation details. |
| GET | /v1/auth/generate | [get auth ticket](#get-auth-ticket) | Generate blobber authentication ticket. |
| GET | /challenge-timings-by-challengeId | [get challenge timing by challenge ID](#get-challenge-timing-by-challenge-id) | Get challenge timing by challenge ID. |
| GET | /challengetimings | [get challenge timings](#get-challenge-timings) | Get challenge timings. |
| GET | /v1/file/download/{allocation} | [get download file](#get-download-file) | Download a file. |
| GET | /v1/file/meta/{allocation} | [get file meta](#get-file-meta) | Get file meta data. |
| GET | /v1/file/stats/{allocation} | [get file stats](#get-file-stats) | Get file stats. |
| GET | /v1/file/latestwritemarker/{allocation} | [get latest write marker](#get-latest-write-marker) | Get latest write marker. |
| GET | /v1/file/list/{allocation} | [get list files](#get-list-files) | List files. |
| GET | /v1/marketplace/shareinfo/{allocation} | [get list share info](#get-list-share-info) | List shared files. |
| GET | /v1/file/objecttree/{allocation} | [get object tree](#get-object-tree) | Get path object tree. |
| GET | /v1/playlist/latest/{allocation} | [get playlist](#get-playlist) | Get playlist. |
| GET | /v1/playlist/file/{allocation} | [get playlist file](#get-playlist-file) | Get playlist file. |
| GET | /v1/file/refs/recent/{allocation} | [get recent refs](#get-recent-refs) | Get recent references. |
| GET | /v1/file/referencepath/{allocation} | [get reference path](#get-reference-path) | Get reference path. |
| GET | /v1/file/refs/{allocation} | [get refs](#get-refs) | Get references. |
| POST | /v1/connection/commit/{allocation} | [post commit](#post-commit) | Commit operation. |
| POST | /v1/connection/create/{allocation} | [post connection](#post-connection) | Store connection in DB. |
| POST | /v1/file/copy/{allocation} | [post copy](#post-copy) | Copy a file. |
| POST | /v1/dir/{allocation} | [post create dir](#post-create-dir) | Create a directory. |
| POST | /v1/writemarker/lock/{allocation} | [post lock write marker](#post-lock-write-marker) | Lock a write marker. |
| POST | /v1/file/move/{allocation} | [post move](#post-move) | Move a file. |
| POST | /v1/connection/redeem/{allocation} | [post redeem](#post-redeem) | Redeem conncetion. |
| POST | /v1/file/rename/{allocation} | [post rename](#post-rename) | Rename file. |
| POST | /v1/connection/rollback/{allocation} | [post rollback](#post-rollback) | Rollback operation. |
| POST | /v1/marketplace/shareinfo/{allocation} | [post share info](#post-share-info) | Share a file. |
| POST | /v1/file/upload/{allocation} | [post upload file](#post-upload-file) | Upload a file. |
| PUT | /v1/file/upload/{allocation} | [put update file](#put-update-file) | Update/Replace a file. |
  


## Paths

### <span id="delete-file"></span> Delete a file. (*DeleteFile*)

```
DELETE /v1/file/upload/{allocation}
```

DeleteHandler is the handler to respond to delete requests from clients. The allocation should permit delete for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| connection_id | `query` | string | `string` |  | ✓ |  | ID of the connection related to this process. Check 2-PC documentation. |
| path | `query` | string | `string` |  | ✓ |  | Path of the file to be deleted. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#delete-file-200) | OK | UploadResult |  | [schema](#delete-file-200-schema) |
| [400](#delete-file-400) | Bad Request |  |  | [schema](#delete-file-400-schema) |
| [500](#delete-file-500) | Internal Server Error |  |  | [schema](#delete-file-500-schema) |

#### Responses


##### <span id="delete-file-200"></span> 200 - UploadResult
Status: OK

###### <span id="delete-file-200-schema"></span> Schema
   
  

[UploadResult](#upload-result)

##### <span id="delete-file-400"></span> 400
Status: Bad Request

###### <span id="delete-file-400-schema"></span> Schema

##### <span id="delete-file-500"></span> 500
Status: Internal Server Error

###### <span id="delete-file-500-schema"></span> Schema

### <span id="delete-lock-write-marker"></span> Unlock a write marker. (*DeleteLockWriteMarker*)

```
DELETE /v1/writemarker/lock/{allocation}/{connection}
```

UnlockWriteMarker release WriteMarkerMutex locked by the Write Marker Lock endpoint.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | allocation id |
| connection | `path` | string | `string` |  | ✓ |  | connection id associae with the write marker |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#delete-lock-write-marker-200) | OK |  |  | [schema](#delete-lock-write-marker-200-schema) |
| [400](#delete-lock-write-marker-400) | Bad Request |  |  | [schema](#delete-lock-write-marker-400-schema) |
| [500](#delete-lock-write-marker-500) | Internal Server Error |  |  | [schema](#delete-lock-write-marker-500-schema) |

#### Responses


##### <span id="delete-lock-write-marker-200"></span> 200
Status: OK

###### <span id="delete-lock-write-marker-200-schema"></span> Schema

##### <span id="delete-lock-write-marker-400"></span> 400
Status: Bad Request

###### <span id="delete-lock-write-marker-400-schema"></span> Schema

##### <span id="delete-lock-write-marker-500"></span> 500
Status: Internal Server Error

###### <span id="delete-lock-write-marker-500-schema"></span> Schema

### <span id="delete-share"></span> Revokes access to a shared file. (*DeleteShare*)

```
DELETE /v1/marketplace/shareinfo/{allocation}
```

Handle revoke share requests from clients.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | TxHash of the allocation in question. |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  | ✓ |  | Digital signature of the client used to verify the request. |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request. Overrides X-App-Client-Signature if provided. |
| path | `query` | string | `string` |  | ✓ |  | Path of the file to be shared. |
| refereeClientID | `query` | string | `string` |  |  |  | The ID of the client to revoke access to the file (in case of private sharing). |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#delete-share-200) | OK |  |  | [schema](#delete-share-200-schema) |
| [400](#delete-share-400) | Bad Request |  |  | [schema](#delete-share-400-schema) |

#### Responses


##### <span id="delete-share-200"></span> 200
Status: OK

###### <span id="delete-share-200-schema"></span> Schema

##### <span id="delete-share-400"></span> 400
Status: Bad Request

###### <span id="delete-share-400-schema"></span> Schema

### <span id="get-allocation"></span> Get allocation details. (*GetAllocation*)

```
GET /allocation
```

Retrieve allocation details as stored in the blobber.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| id | `query` | string | `string` |  | ✓ |  | allocation ID |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-allocation-200) | OK | Allocation |  | [schema](#get-allocation-200-schema) |
| [400](#get-allocation-400) | Bad Request |  |  | [schema](#get-allocation-400-schema) |
| [500](#get-allocation-500) | Internal Server Error |  |  | [schema](#get-allocation-500-schema) |

#### Responses


##### <span id="get-allocation-200"></span> 200 - Allocation
Status: OK

###### <span id="get-allocation-200-schema"></span> Schema
   
  

[Allocation](#allocation)

##### <span id="get-allocation-400"></span> 400
Status: Bad Request

###### <span id="get-allocation-400-schema"></span> Schema

##### <span id="get-allocation-500"></span> 500
Status: Internal Server Error

###### <span id="get-allocation-500-schema"></span> Schema

### <span id="get-auth-ticket"></span> Generate blobber authentication ticket. (*GetAuthTicket*)

```
GET /v1/auth/generate
```

Generate and retrieve blobber authentication ticket signed by the blobber's signature. Used by restricted blobbers to enable users to use them to host allocations.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| Zbox-Signature | `header` | string | `string` |  |  |  | Digital signature to verify that the sender is 0box service. |
| client_id | `query` | string | `string` |  |  |  | Client ID is used as a payload to the token generated. The token represents a signed version of this string by the blobber's private key. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-auth-ticket-200) | OK | AuthTicketResponse |  | [schema](#get-auth-ticket-200-schema) |

#### Responses


##### <span id="get-auth-ticket-200"></span> 200 - AuthTicketResponse
Status: OK

###### <span id="get-auth-ticket-200-schema"></span> Schema
   
  

[AuthTicketResponse](#auth-ticket-response)

### <span id="get-challenge-timing-by-challenge-id"></span> Get challenge timing by challenge ID. (*GetChallengeTimingByChallengeID*)

```
GET /challenge-timings-by-challengeId
```

Retrieve challenge timing for the given challenge ID by the blobber admin.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| Authorization | `header` | string | `string` |  | ✓ |  | Authorization header (Basic auth). MUST be provided to fulfil the request |
| challenge_id | `query` | string | `string` |  | ✓ |  | Challenge ID for which to retrieve the challenge timing |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-challenge-timing-by-challenge-id-200) | OK | ChallengeTiming |  | [schema](#get-challenge-timing-by-challenge-id-200-schema) |

#### Responses


##### <span id="get-challenge-timing-by-challenge-id-200"></span> 200 - ChallengeTiming
Status: OK

###### <span id="get-challenge-timing-by-challenge-id-200-schema"></span> Schema
   
  

[ChallengeTiming](#challenge-timing)

### <span id="get-challenge-timings"></span> Get challenge timings. (*GetChallengeTimings*)

```
GET /challengetimings
```

Retrieve challenge timings for the blobber admin.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| Authorization | `header` | string | `string` |  | ✓ |  | Authorization header (Basic auth). MUST be provided to fulfil the request |
| from | `query` | integer | `int64` |  |  |  | An optional timestamp from which to retrieve the challenge timings |
| limit | `query` | integer | `int64` |  |  |  | Pagination limit, number of entries in the page to retrieve. Default is 20. |
| offset | `query` | integer | `int64` |  |  |  | Pagination offset, start of the page to retrieve. Default is 0. |
| sort | `query` | string | `string` |  |  |  | Direction of sorting based on challenge closure time, either "asc" or "desc". Default is "asc" |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-challenge-timings-200) | OK | ChallengeTiming |  | [schema](#get-challenge-timings-200-schema) |

#### Responses


##### <span id="get-challenge-timings-200"></span> 200 - ChallengeTiming
Status: OK

###### <span id="get-challenge-timings-200-schema"></span> Schema
   
  

[][ChallengeTiming](#challenge-timing)

### <span id="get-download-file"></span> Download a file. (*GetDownloadFile*)

```
GET /v1/file/download/{allocation}
```

Download Handler (downloadFile). The response is either a byte stream or a FileDownloadResponse, which contains the file data or the thumbnail data, and the merkle proof if the download is verified.
This depends on the "X-Verify-Download" header. If the header is set to "true", the response is a FileDownloadResponse, otherwise it is a byte stream.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | TxHash of the allocation in question. |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| X-Auth-Token | `header` | string | `string` |  |  |  | The auth token to use for the download. If the file is shared, the auth token is required. |
| X-Block-Num | `header` | integer | `int64` |  |  |  | The block number of the file to download. Must be 0 or greater (valid index). |
| X-Connection-ID | `header` | string | `string` |  |  |  | The ID of the connection used for the download. Usually, the download process occurs in multiple requests, on per block, where all of them are done in a single connection between the client and the blobber. |
| X-Mode | `header` | string | `string` |  |  |  | Download mode. Either "full" for full file download, or "thumbnail" to download the thumbnail of the file |
| X-Num-Blocks | `header` | integer | `int64` |  |  |  | The number of blocks to download. Must be 0 or greater. |
| X-Path | `header` | string | `string` |  | ✓ |  | The path of the file to download. |
| X-Path-Hash | `header` | string | `string` |  |  |  | The hash of the path of the file to download. If not provided, will be calculated from "X-Path" parameter. |
| X-Read-Marker | `header` | string | `string` |  |  |  | The read marker to use for the download (check [ReadMarker](#/responses/ReadMarker)). |
| X-Verify-Download | `header` | string | `string` |  |  |  | If set to "true", the download should be verified. If the mode is "thumbnail", the thumbnail hash stored in the db is compared with the hash of the actual file. If the mode is "full", merkle proof is calculated and returned in the response. |
| X-Version | `header` | string | `string` |  |  |  | If its value is "v2" then both allocation_id and blobber url base are hashed and verified using X-App-Client-Signature-V2. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-download-file-200) | OK | FileDownloadResponse |  | [schema](#get-download-file-200-schema) |
| [400](#get-download-file-400) | Bad Request |  |  | [schema](#get-download-file-400-schema) |

#### Responses


##### <span id="get-download-file-200"></span> 200 - FileDownloadResponse
Status: OK

###### <span id="get-download-file-200-schema"></span> Schema
   
  

[FileDownloadResponse](#file-download-response)

##### <span id="get-download-file-400"></span> 400
Status: Bad Request

###### <span id="get-download-file-400-schema"></span> Schema

### <span id="get-file-meta"></span> Get file meta data. (*GetFileMeta*)

```
GET /v1/file/meta/{allocation}
```

Retrieve file meta data from the blobber. Retrieves a generic map of string keys and values.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| auth_token | `query` | string | `string` |  |  |  | The auth ticket for the file to show meta data of if the client does not own it. Check File Sharing docs for more info. |
| name | `query` | string | `string` |  |  |  | the name of the file |
| path | `query` | string | `string` |  |  |  | Path of the file to be copied. Required only if `path_hash` is not provided. |
| path_hash | `query` | string | `string` |  |  |  | Hash of the path of the file to be copied. Required only if `path` is not provided. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-file-meta-200) | OK |  |  | [schema](#get-file-meta-200-schema) |
| [400](#get-file-meta-400) | Bad Request |  |  | [schema](#get-file-meta-400-schema) |
| [500](#get-file-meta-500) | Internal Server Error |  |  | [schema](#get-file-meta-500-schema) |

#### Responses


##### <span id="get-file-meta-200"></span> 200
Status: OK

###### <span id="get-file-meta-200-schema"></span> Schema

##### <span id="get-file-meta-400"></span> 400
Status: Bad Request

###### <span id="get-file-meta-400-schema"></span> Schema

##### <span id="get-file-meta-500"></span> 500
Status: Internal Server Error

###### <span id="get-file-meta-500-schema"></span> Schema

### <span id="get-file-stats"></span> Get file stats. (*GetFileStats*)

```
GET /v1/file/stats/{allocation}
```

Retrieve file stats from the blobber.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| path | `query` | string | `string` |  |  |  | Path of the file to be copied. Required only if `path_hash` is not provided. |
| path_hash | `query` | string | `string` |  |  |  | Hash of the path of the file to be copied. Required only if `path` is not provided. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-file-stats-200) | OK | FileStats |  | [schema](#get-file-stats-200-schema) |
| [400](#get-file-stats-400) | Bad Request |  |  | [schema](#get-file-stats-400-schema) |
| [500](#get-file-stats-500) | Internal Server Error |  |  | [schema](#get-file-stats-500-schema) |

#### Responses


##### <span id="get-file-stats-200"></span> 200 - FileStats
Status: OK

###### <span id="get-file-stats-200-schema"></span> Schema
   
  

[FileStats](#file-stats)

##### <span id="get-file-stats-400"></span> 400
Status: Bad Request

###### <span id="get-file-stats-400-schema"></span> Schema

##### <span id="get-file-stats-500"></span> 500
Status: Internal Server Error

###### <span id="get-file-stats-500-schema"></span> Schema

### <span id="get-latest-write-marker"></span> Get latest write marker. (*GetLatestWriteMarker*)

```
GET /v1/file/latestwritemarker/{allocation}
```

Retrieve the latest write marker associated with the allocation

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-latest-write-marker-200) | OK | LatestWriteMarkerResult |  | [schema](#get-latest-write-marker-200-schema) |
| [400](#get-latest-write-marker-400) | Bad Request |  |  | [schema](#get-latest-write-marker-400-schema) |
| [500](#get-latest-write-marker-500) | Internal Server Error |  |  | [schema](#get-latest-write-marker-500-schema) |

#### Responses


##### <span id="get-latest-write-marker-200"></span> 200 - LatestWriteMarkerResult
Status: OK

###### <span id="get-latest-write-marker-200-schema"></span> Schema
   
  

[LatestWriteMarkerResult](#latest-write-marker-result)

##### <span id="get-latest-write-marker-400"></span> 400
Status: Bad Request

###### <span id="get-latest-write-marker-400-schema"></span> Schema

##### <span id="get-latest-write-marker-500"></span> 500
Status: Internal Server Error

###### <span id="get-latest-write-marker-500-schema"></span> Schema

### <span id="get-list-files"></span> List files. (*GetListFiles*)

```
GET /v1/file/list/{allocation}
```

ListHandler is the handler to respond to list requests from clients, 
it returns a list of files in the allocation,
along with the metadata of the files.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | TxHash of the allocation in question. |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| auth_token | `query` | string | `string` |  |  |  | The auth ticket for the file to download if the client does not own it. Check File Sharing docs for more info. |
| limit | `query` | integer | `int64` |  | ✓ |  | The number of files to return (for pagination). |
| list | `query` | boolean | `bool` |  |  |  | Whether or not to list the files inside the directory, not just data about the path itself. |
| offset | `query` | integer | `int64` |  | ✓ |  | The number of files to skip before returning (for pagination). |
| path | `query` | string | `string` |  | ✓ |  | The path needed to list info about |
| path_hash | `query` | string | `string` |  |  |  | Lookuphash of the path needed to list info about, which is a hex hash of the path concatenated with the allocation ID. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-list-files-200) | OK | ListResult |  | [schema](#get-list-files-200-schema) |
| [400](#get-list-files-400) | Bad Request |  |  | [schema](#get-list-files-400-schema) |

#### Responses


##### <span id="get-list-files-200"></span> 200 - ListResult
Status: OK

###### <span id="get-list-files-200-schema"></span> Schema
   
  

[ListResult](#list-result)

##### <span id="get-list-files-400"></span> 400
Status: Bad Request

###### <span id="get-list-files-400-schema"></span> Schema

### <span id="get-list-share-info"></span> List shared files. (*GetListShareInfo*)

```
GET /v1/marketplace/shareinfo/{allocation}
```

Retrieve shared files in an allocation by its owner.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| limit | `query` | integer | `int64` |  |  |  | Pagination limit, number of entries in the page to retrieve. Default is 20. |
| offset | `query` | integer | `int64` |  |  |  | Pagination offset, start of the page to retrieve. Default is 0. |
| sort | `query` | string | `string` |  |  |  | Direction of sorting based on challenge closure time, either "asc" or "desc". Default is "asc" |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-list-share-info-200) | OK | ShareInfo |  | [schema](#get-list-share-info-200-schema) |
| [400](#get-list-share-info-400) | Bad Request |  |  | [schema](#get-list-share-info-400-schema) |
| [500](#get-list-share-info-500) | Internal Server Error |  |  | [schema](#get-list-share-info-500-schema) |

#### Responses


##### <span id="get-list-share-info-200"></span> 200 - ShareInfo
Status: OK

###### <span id="get-list-share-info-200-schema"></span> Schema
   
  

[][ShareInfo](#share-info)

##### <span id="get-list-share-info-400"></span> 400
Status: Bad Request

###### <span id="get-list-share-info-400-schema"></span> Schema

##### <span id="get-list-share-info-500"></span> 500
Status: Internal Server Error

###### <span id="get-list-share-info-500-schema"></span> Schema

### <span id="get-object-tree"></span> Get path object tree. (*GetObjectTree*)

```
GET /v1/file/objecttree/{allocation}
```

Retrieve object tree reference path. Similar to reference path.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| path | `query` | string | `string` |  |  |  | Path of the file needed to get reference path of. Required only if no "paths" are provided. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-object-tree-200) | OK | ReferencePathResult |  | [schema](#get-object-tree-200-schema) |
| [400](#get-object-tree-400) | Bad Request |  |  | [schema](#get-object-tree-400-schema) |
| [500](#get-object-tree-500) | Internal Server Error |  |  | [schema](#get-object-tree-500-schema) |

#### Responses


##### <span id="get-object-tree-200"></span> 200 - ReferencePathResult
Status: OK

###### <span id="get-object-tree-200-schema"></span> Schema
   
  

[ReferencePathResult](#reference-path-result)

##### <span id="get-object-tree-400"></span> 400
Status: Bad Request

###### <span id="get-object-tree-400-schema"></span> Schema

##### <span id="get-object-tree-500"></span> 500
Status: Internal Server Error

###### <span id="get-object-tree-500-schema"></span> Schema

### <span id="get-playlist"></span> Get playlist. (*GetPlaylist*)

```
GET /v1/playlist/latest/{allocation}
```

Loads playlist from a given path in an allocation.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | allocation id |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| auth_token | `query` | string | `string` |  |  |  | The auth token to access the playlist. This is required when the playlist is accessed by a non-owner of the allocation. |
| lookup_hash | `query` | string | `string` |  |  |  | The lookup hash of the file for which the playlist is to be retrieved. This is required when the playlist is accessed by a non-owner of the allocation. |
| path | `query` | string | `string` |  |  |  | The path of the file for which the playlist is to be retrieved. This is required when the playlist is accessed by the owner of the allocation. |
| since | `query` | string | `string` |  |  |  | The lookup hash of the file from which to start the playlist. The retrieved playlist will start from the id associated with this lookup hash and going forward. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-playlist-200) | OK | PlaylistFile |  | [schema](#get-playlist-200-schema) |
| [400](#get-playlist-400) | Bad Request |  |  | [schema](#get-playlist-400-schema) |
| [500](#get-playlist-500) | Internal Server Error |  |  | [schema](#get-playlist-500-schema) |

#### Responses


##### <span id="get-playlist-200"></span> 200 - PlaylistFile
Status: OK

###### <span id="get-playlist-200-schema"></span> Schema
   
  

[][PlaylistFile](#playlist-file)

##### <span id="get-playlist-400"></span> 400
Status: Bad Request

###### <span id="get-playlist-400-schema"></span> Schema

##### <span id="get-playlist-500"></span> 500
Status: Internal Server Error

###### <span id="get-playlist-500-schema"></span> Schema

### <span id="get-playlist-file"></span> Get playlist file. (*GetPlaylistFile*)

```
GET /v1/playlist/file/{allocation}
```

Loads the metadata of a the playlist file with the given lookup hash.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | allocation id |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| auth_token | `query` | string | `string` |  |  |  | The auth token to access the playlist. This is required when the playlist is accessed by a non-owner of the allocation. |
| lookup_hash | `query` | string | `string` |  |  |  | The lookup hash of the file for which the playlist is to be retrieved. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-playlist-file-200) | OK | PlaylistFile |  | [schema](#get-playlist-file-200-schema) |
| [400](#get-playlist-file-400) | Bad Request |  |  | [schema](#get-playlist-file-400-schema) |
| [500](#get-playlist-file-500) | Internal Server Error |  |  | [schema](#get-playlist-file-500-schema) |

#### Responses


##### <span id="get-playlist-file-200"></span> 200 - PlaylistFile
Status: OK

###### <span id="get-playlist-file-200-schema"></span> Schema
   
  

[PlaylistFile](#playlist-file)

##### <span id="get-playlist-file-400"></span> 400
Status: Bad Request

###### <span id="get-playlist-file-400-schema"></span> Schema

##### <span id="get-playlist-file-500"></span> 500
Status: Internal Server Error

###### <span id="get-playlist-file-500-schema"></span> Schema

### <span id="get-recent-refs"></span> Get recent references. (*GetRecentRefs*)

```
GET /v1/file/refs/recent/{allocation}
```

Retrieve recent references added to an allocation, starting at a specific date, organized in a paginated table.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| from-date | `query` | integer | `int64` |  |  |  | Timestamp to start listing from. Ignored if not provided. |
| limit | `query` | integer | `int64` |  |  |  | Number of records to show per page. If provided more than 100, it will be set to 100. Default is 20. |
| offset | `query` | string | `string` |  |  |  | Pagination offset. Default is 0. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-recent-refs-200) | OK | RecentRefResult |  | [schema](#get-recent-refs-200-schema) |
| [400](#get-recent-refs-400) | Bad Request |  |  | [schema](#get-recent-refs-400-schema) |
| [500](#get-recent-refs-500) | Internal Server Error |  |  | [schema](#get-recent-refs-500-schema) |

#### Responses


##### <span id="get-recent-refs-200"></span> 200 - RecentRefResult
Status: OK

###### <span id="get-recent-refs-200-schema"></span> Schema
   
  

[RecentRefResult](#recent-ref-result)

##### <span id="get-recent-refs-400"></span> 400
Status: Bad Request

###### <span id="get-recent-refs-400-schema"></span> Schema

##### <span id="get-recent-refs-500"></span> 500
Status: Internal Server Error

###### <span id="get-recent-refs-500-schema"></span> Schema

### <span id="get-reference-path"></span> Get reference path. (*GetReferencePath*)

```
GET /v1/file/referencepath/{allocation}
```

Retrieve references of all the decendents of a given path including itself, known as reference path. Reference (shorted as Ref) is the representation of a certain path in the DB including its metadata.
It also returns the latest write marker associated with the allocation.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| path | `query` | string | `string` |  |  |  | Path of the file needed to get reference path of. Required only if no "paths" are provided. |
| paths | `query` | string | `string` |  |  |  | Paths of the files needed to get reference path of. Required only if no "path" is provided. Should be provided as valid JSON array. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-reference-path-200) | OK | ReferencePathResult |  | [schema](#get-reference-path-200-schema) |
| [400](#get-reference-path-400) | Bad Request |  |  | [schema](#get-reference-path-400-schema) |
| [500](#get-reference-path-500) | Internal Server Error |  |  | [schema](#get-reference-path-500-schema) |

#### Responses


##### <span id="get-reference-path-200"></span> 200 - ReferencePathResult
Status: OK

###### <span id="get-reference-path-200-schema"></span> Schema
   
  

[ReferencePathResult](#reference-path-result)

##### <span id="get-reference-path-400"></span> 400
Status: Bad Request

###### <span id="get-reference-path-400-schema"></span> Schema

##### <span id="get-reference-path-500"></span> 500
Status: Internal Server Error

###### <span id="get-reference-path-500-schema"></span> Schema

### <span id="get-refs"></span> Get references. (*GetRefs*)

```
GET /v1/file/refs/{allocation}
```

Retrieve references of all the decendents of a given path including itself, organized in a paginated table.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| auth_token | `query` | string | `string` |  |  |  | The auth ticket for the file to show meta data of if the client does not own it. Check File Sharing docs for more info. |
| fileType | `query` | string | `string` |  |  |  | Type of the references to list. Can be "f" for file or "d" for directory. Both will be retrieved if not provided. |
| level | `query` | integer | `int64` |  |  |  | Level of the references to list (number of parents of the reference). Can be "0" for root level or "1" for first level and so on. All levels will be retrieved if not provided. |
| offsetDate | `query` | string | `string` |  |  |  | Date of the file to start the listing from.  Used in case the user needs to list refs updated at some period of time. |
| offsetPath | `query` | string | `string` |  |  |  | Path of the file to start the listing from. Used for pagination. |
| pageLimit | `query` | integer | `int64` |  |  |  | Number of records to show per page. Default is 20. |
| path | `query` | string | `string` |  |  |  | Path of the file needed to get reference path of. Required only if no "paths" are provided. |
| path_hash | `query` | string | `string` |  |  |  | Hash of the path of the file to be copied. Required only if `path` is not provided. |
| refType | `query` | string | `string` |  | ✓ |  | Can be "updated" (along with providing `updateDate` and `offsetDate`) to retrieve refs with updated_at date later than the provided date in both fields, or "regular" otherwise. |
| updateDate | `query` | string | `string` |  |  |  | Same as offsetDate but both should be provided. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#get-refs-200) | OK | RefResult |  | [schema](#get-refs-200-schema) |
| [400](#get-refs-400) | Bad Request |  |  | [schema](#get-refs-400-schema) |
| [500](#get-refs-500) | Internal Server Error |  |  | [schema](#get-refs-500-schema) |

#### Responses


##### <span id="get-refs-200"></span> 200 - RefResult
Status: OK

###### <span id="get-refs-200-schema"></span> Schema
   
  

[RefResult](#ref-result)

##### <span id="get-refs-400"></span> 400
Status: Bad Request

###### <span id="get-refs-400-schema"></span> Schema

##### <span id="get-refs-500"></span> 500
Status: Internal Server Error

###### <span id="get-refs-500-schema"></span> Schema

### <span id="post-commit"></span> Commit operation. (*PostCommit*)

```
POST /v1/connection/commit/{allocation}
```

Used to commit the storage operation provided its connection id.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| connection_id | `query` | string | `string` |  | ✓ |  | the connection ID of the storage operation to commit |
| write_marker | `query` | string | `string` |  | ✓ |  | The write marker corresponding to the operation. Write price is used to redeem storage cost from the network. It follows the format of the [Write Marker](#write-marker) |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#post-commit-200) | OK | CommitResult |  | [schema](#post-commit-200-schema) |
| [400](#post-commit-400) | Bad Request |  |  | [schema](#post-commit-400-schema) |
| [500](#post-commit-500) | Internal Server Error |  |  | [schema](#post-commit-500-schema) |

#### Responses


##### <span id="post-commit-200"></span> 200 - CommitResult
Status: OK

###### <span id="post-commit-200-schema"></span> Schema
   
  

[CommitResult](#commit-result)

##### <span id="post-commit-400"></span> 400
Status: Bad Request

###### <span id="post-commit-400-schema"></span> Schema

##### <span id="post-commit-500"></span> 500
Status: Internal Server Error

###### <span id="post-commit-500-schema"></span> Schema

### <span id="post-connection"></span> Store connection in DB. (*PostConnection*)

```
POST /v1/connection/create/{allocation}
```

Connections are used to distinguish between different storage operations, also to claim reward from the chain using write markers.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| connection_id | `query` | string | `string` |  | ✓ |  | the ID of the connection to submit. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#post-connection-200) | OK | ConnectionResult |  | [schema](#post-connection-200-schema) |
| [400](#post-connection-400) | Bad Request |  |  | [schema](#post-connection-400-schema) |
| [500](#post-connection-500) | Internal Server Error |  |  | [schema](#post-connection-500-schema) |

#### Responses


##### <span id="post-connection-200"></span> 200 - ConnectionResult
Status: OK

###### <span id="post-connection-200-schema"></span> Schema
   
  

[ConnectionResult](#connection-result)

##### <span id="post-connection-400"></span> 400
Status: Bad Request

###### <span id="post-connection-400-schema"></span> Schema

##### <span id="post-connection-500"></span> 500
Status: Internal Server Error

###### <span id="post-connection-500-schema"></span> Schema

### <span id="post-copy"></span> Copy a file. (*PostCopy*)

```
POST /v1/file/copy/{allocation}
```

Copy a file in an allocation. Can only be run by the owner of the allocation.
The allocation should permit copy for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| connection_id | `query` | string | `string` |  | ✓ |  | Connection ID related to this process. Blobber uses the connection id to redeem rewards for storage operations and distinguish the operation. Connection should be using the create connection endpoint. |
| dest | `query` | string | `string` |  | ✓ |  | Destination path of the file to be copied. |
| path | `query` | string | `string` |  |  |  | Path of the file to be copied. Required only if `path_hash` is not provided. |
| path_hash | `query` | string | `string` |  |  |  | Hash of the path of the file to be copied. Required only if `path` is not provided. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#post-copy-200) | OK | UploadResult |  | [schema](#post-copy-200-schema) |
| [400](#post-copy-400) | Bad Request |  |  | [schema](#post-copy-400-schema) |
| [500](#post-copy-500) | Internal Server Error |  |  | [schema](#post-copy-500-schema) |

#### Responses


##### <span id="post-copy-200"></span> 200 - UploadResult
Status: OK

###### <span id="post-copy-200-schema"></span> Schema
   
  

[UploadResult](#upload-result)

##### <span id="post-copy-400"></span> 400
Status: Bad Request

###### <span id="post-copy-400-schema"></span> Schema

##### <span id="post-copy-500"></span> 500
Status: Internal Server Error

###### <span id="post-copy-500-schema"></span> Schema

### <span id="post-create-dir"></span> Create a directory. (*PostCreateDir*)

```
POST /v1/dir/{allocation}
```

Creates a directory in an allocation. Can only be run by the owner of the allocation.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| dir_path | `query` | string | `string` |  | ✓ |  | Path of the directory to be created. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#post-create-dir-200) | OK | UploadResult |  | [schema](#post-create-dir-200-schema) |
| [400](#post-create-dir-400) | Bad Request |  |  | [schema](#post-create-dir-400-schema) |
| [500](#post-create-dir-500) | Internal Server Error |  |  | [schema](#post-create-dir-500-schema) |

#### Responses


##### <span id="post-create-dir-200"></span> 200 - UploadResult
Status: OK

###### <span id="post-create-dir-200-schema"></span> Schema
   
  

[UploadResult](#upload-result)

##### <span id="post-create-dir-400"></span> 400
Status: Bad Request

###### <span id="post-create-dir-400-schema"></span> Schema

##### <span id="post-create-dir-500"></span> 500
Status: Internal Server Error

###### <span id="post-create-dir-500-schema"></span> Schema

### <span id="post-lock-write-marker"></span> Lock a write marker. (*PostLockWriteMarker*)

```
POST /v1/writemarker/lock/{allocation}
```

LockWriteMarker try to lock writemarker for specified allocation id.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | allocation id |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| connection_id | `query` | string | `string` |  | ✓ |  | The ID of the connection associated with the write marker. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#post-lock-write-marker-200) | OK | WriteMarkerLockResult |  | [schema](#post-lock-write-marker-200-schema) |
| [400](#post-lock-write-marker-400) | Bad Request |  |  | [schema](#post-lock-write-marker-400-schema) |
| [500](#post-lock-write-marker-500) | Internal Server Error |  |  | [schema](#post-lock-write-marker-500-schema) |

#### Responses


##### <span id="post-lock-write-marker-200"></span> 200 - WriteMarkerLockResult
Status: OK

###### <span id="post-lock-write-marker-200-schema"></span> Schema
   
  

[LockResult](#lock-result)

##### <span id="post-lock-write-marker-400"></span> 400
Status: Bad Request

###### <span id="post-lock-write-marker-400-schema"></span> Schema

##### <span id="post-lock-write-marker-500"></span> 500
Status: Internal Server Error

###### <span id="post-lock-write-marker-500-schema"></span> Schema

### <span id="post-move"></span> Move a file. (*PostMove*)

```
POST /v1/file/move/{allocation}
```

Mova a file from a path to another in an allocation. Can only be run by the owner of the allocation.
The allocation should permit move for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| connection_id | `query` | string | `string` |  | ✓ |  | Connection ID related to this process. Blobber uses the connection id to redeem rewards for storage operations and distinguish the operation. Connection should be using the create connection endpoint. |
| dest | `query` | string | `string` |  | ✓ |  | Destination path of the file to be moved. |
| path | `query` | string | `string` |  |  |  | Path of the file to be moved. Required only if `path_hash` is not provided. |
| path_hash | `query` | string | `string` |  |  |  | Hash of the path of the file to be moved. Required only if `path` is not provided. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#post-move-200) | OK | UploadResult |  | [schema](#post-move-200-schema) |
| [400](#post-move-400) | Bad Request |  |  | [schema](#post-move-400-schema) |
| [500](#post-move-500) | Internal Server Error |  |  | [schema](#post-move-500-schema) |

#### Responses


##### <span id="post-move-200"></span> 200 - UploadResult
Status: OK

###### <span id="post-move-200-schema"></span> Schema
   
  

[UploadResult](#upload-result)

##### <span id="post-move-400"></span> 400
Status: Bad Request

###### <span id="post-move-400-schema"></span> Schema

##### <span id="post-move-500"></span> 500
Status: Internal Server Error

###### <span id="post-move-500-schema"></span> Schema

### <span id="post-redeem"></span> Redeem conncetion. (*PostRedeem*)

```
POST /v1/connection/redeem/{allocation}
```

Submit the connection ID to redeem the storage cost from the network.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#post-redeem-200) | OK | DownloadResponse |  | [schema](#post-redeem-200-schema) |
| [400](#post-redeem-400) | Bad Request |  |  | [schema](#post-redeem-400-schema) |
| [500](#post-redeem-500) | Internal Server Error |  |  | [schema](#post-redeem-500-schema) |

#### Responses


##### <span id="post-redeem-200"></span> 200 - DownloadResponse
Status: OK

###### <span id="post-redeem-200-schema"></span> Schema
   
  

[DownloadResponse](#download-response)

##### <span id="post-redeem-400"></span> 400
Status: Bad Request

###### <span id="post-redeem-400-schema"></span> Schema

##### <span id="post-redeem-500"></span> 500
Status: Internal Server Error

###### <span id="post-redeem-500-schema"></span> Schema

### <span id="post-rename"></span> Rename file. (*PostRename*)

```
POST /v1/file/rename/{allocation}
```

Rename a file in an allocation. Can only be run by the owner of the allocation.
The allocation should permit rename for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| connection_id | `query` | string | `string` |  | ✓ |  | Connection ID related to this process. Blobber uses the connection id to redeem rewards for storage and distinguish the operation. Connection should be using the create connection endpoint. |
| new_name | `query` | string | `string` |  | ✓ |  | Name to be set to the file/directory. |
| path | `query` | string | `string` |  |  |  | Path of the file to be renamed. Required only if `path_hash` is not provided. |
| path_hash | `query` | string | `string` |  |  |  | Hash of the path of the file to be renamed. Required only if `path` is not provided. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#post-rename-200) | OK | UploadResult |  | [schema](#post-rename-200-schema) |
| [400](#post-rename-400) | Bad Request |  |  | [schema](#post-rename-400-schema) |
| [500](#post-rename-500) | Internal Server Error |  |  | [schema](#post-rename-500-schema) |

#### Responses


##### <span id="post-rename-200"></span> 200 - UploadResult
Status: OK

###### <span id="post-rename-200-schema"></span> Schema
   
  

[UploadResult](#upload-result)

##### <span id="post-rename-400"></span> 400
Status: Bad Request

###### <span id="post-rename-400-schema"></span> Schema

##### <span id="post-rename-500"></span> 500
Status: Internal Server Error

###### <span id="post-rename-500-schema"></span> Schema

### <span id="post-rollback"></span> Rollback operation. (*PostRollback*)

```
POST /v1/connection/rollback/{allocation}
```

RollbackHandler used to commit the storage operation provided its connection id.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| connection_id | `query` | string | `string` |  | ✓ |  | the connection ID of the storage operation to rollback |
| write_marker | `query` | string | `string` |  | ✓ |  | The write marker corresponding to the operation. Write price is used to redeem storage cost from the network. It follows the format of the [Write Marker](#write-marker) |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#post-rollback-200) | OK | CommitResult |  | [schema](#post-rollback-200-schema) |
| [400](#post-rollback-400) | Bad Request |  |  | [schema](#post-rollback-400-schema) |
| [500](#post-rollback-500) | Internal Server Error |  |  | [schema](#post-rollback-500-schema) |

#### Responses


##### <span id="post-rollback-200"></span> 200 - CommitResult
Status: OK

###### <span id="post-rollback-200-schema"></span> Schema
   
  

[CommitResult](#commit-result)

##### <span id="post-rollback-400"></span> 400
Status: Bad Request

###### <span id="post-rollback-400-schema"></span> Schema

##### <span id="post-rollback-500"></span> 500
Status: Internal Server Error

###### <span id="post-rollback-500-schema"></span> Schema

### <span id="post-share-info"></span> Share a file. (*PostShareInfo*)

```
POST /v1/marketplace/shareinfo/{allocation}
```

Handle share file requests from clients. Returns generic mapping. 

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | TxHash of the allocation in question. |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  | ✓ |  | Digital signature of the client used to verify the request. |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request. Overrides X-App-Client-Signature if provided. |
| auth_ticket | `formData` | string | `string` |  | ✓ |  | Body of the auth ticket used to verify the file access. Follows the structure of [`AuthTicket`](#auth-ticket) |
| available_after | `formData` | string | `string` |  |  |  | Time after which the file will be accessible for sharing. |
| encryption_public_key | `formData` | string | `string` |  |  |  | Public key of the referee client in case of private sharing. Used for proxy re-encryption. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#post-share-info-200) | OK |  |  | [schema](#post-share-info-200-schema) |
| [400](#post-share-info-400) | Bad Request |  |  | [schema](#post-share-info-400-schema) |

#### Responses


##### <span id="post-share-info-200"></span> 200
Status: OK

###### <span id="post-share-info-200-schema"></span> Schema

##### <span id="post-share-info-400"></span> 400
Status: Bad Request

###### <span id="post-share-info-400-schema"></span> Schema

### <span id="post-upload-file"></span> Upload a file. (*PostUploadFile*)

```
POST /v1/file/upload/{allocation}
```

uploadHandler is the handler to respond to upload requests from clients. The allocation should permit upload for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| connection_id | `query` | string | `string` |  | ✓ |  | ID of the connection related to this process. Check 2-PC documentation. |
| uploadFile | `formData` | file | `io.ReadCloser` |  | ✓ |  | File to be uploaded. |
| uploadMeta | `formData` | string | `string` |  | ✓ |  | Metadata of the file to be uploaded. It should be a valid JSON object following the UploadFileChanger schema. |
| uploadThumbnailFile | `formData` | file | `io.ReadCloser` |  |  |  | Thumbnail file to be uploaded. It should be a valid image file. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#post-upload-file-200) | OK | UploadResult |  | [schema](#post-upload-file-200-schema) |
| [400](#post-upload-file-400) | Bad Request |  |  | [schema](#post-upload-file-400-schema) |
| [500](#post-upload-file-500) | Internal Server Error |  |  | [schema](#post-upload-file-500-schema) |

#### Responses


##### <span id="post-upload-file-200"></span> 200 - UploadResult
Status: OK

###### <span id="post-upload-file-200-schema"></span> Schema
   
  

[UploadResult](#upload-result)

##### <span id="post-upload-file-400"></span> 400
Status: Bad Request

###### <span id="post-upload-file-400-schema"></span> Schema

##### <span id="post-upload-file-500"></span> 500
Status: Internal Server Error

###### <span id="post-upload-file-500-schema"></span> Schema

### <span id="put-update-file"></span> Update/Replace a file. (*PutUpdateFile*)

```
PUT /v1/file/upload/{allocation}
```

UpdateHandler is the handler to respond to update requests from clients. The allocation should permit update for this operation to succeed. Check System Features > Storage > File Operations > File Permissions for more info.

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| ALLOCATION-ID | `header` | string | `string` |  | ✓ |  | The ID of the allocation in question. |
| X-App-Client-ID | `header` | string | `string` |  | ✓ |  | The ID/Wallet address of the client sending the request. |
| X-App-Client-Key | `header` | string | `string` |  | ✓ |  | The key of the client sending the request. |
| X-App-Client-Signature | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is not "v2" |
| X-App-Client-Signature-V2 | `header` | string | `string` |  |  |  | Digital signature of the client used to verify the request if the X-Version is "v2" |
| connection_id | `query` | string | `string` |  | ✓ |  | ID of the connection related to this process. Check 2-PC documentation. |
| uploadFile | `formData` | file | `io.ReadCloser` |  | ✓ |  | File to replace the existing one. |
| uploadMeta | `formData` | string | `string` |  | ✓ |  | Metadata of the file to be replaced with the current file. It should be a valid JSON object following the UploadFileChanger schema. |
| uploadThumbnailFile | `formData` | file | `io.ReadCloser` |  |  |  | Thumbnail file to be replaced. It should be a valid image file. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#put-update-file-200) | OK | UploadResult |  | [schema](#put-update-file-200-schema) |
| [400](#put-update-file-400) | Bad Request |  |  | [schema](#put-update-file-400-schema) |
| [500](#put-update-file-500) | Internal Server Error |  |  | [schema](#put-update-file-500-schema) |

#### Responses


##### <span id="put-update-file-200"></span> 200 - UploadResult
Status: OK

###### <span id="put-update-file-200-schema"></span> Schema
   
  

[UploadResult](#upload-result)

##### <span id="put-update-file-400"></span> 400
Status: Bad Request

###### <span id="put-update-file-400-schema"></span> Schema

##### <span id="put-update-file-500"></span> 500
Status: Internal Server Error

###### <span id="put-update-file-500-schema"></span> Schema

## Models

### <span id="allocation"></span> Allocation


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| AllocationRoot | string| `string` |  | | AllocationRoot allcation_root of last write_marker |  |
| BlobberSize | int64 (formatted integer)| `int64` |  | |  |  |
| BlobberSizeUsed | int64 (formatted integer)| `int64` |  | |  |  |
| CleanedUp | boolean| `bool` |  | | Ending and cleaning |  |
| Expiration | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| FileMetaRoot | string| `string` |  | |  |  |
| FileOptions | uint16 (formatted integer)| `uint16` |  | | FileOptions to define file restrictions on an allocation for third-parties</br>default 00000000 for all crud operations suggesting only owner has the below listed abilities.</br>enabling option/s allows any third party to perform certain ops</br>00000001 - 1  - upload</br>00000010 - 2  - delete</br>00000100 - 4  - update</br>00001000 - 8  - move</br>00010000 - 16 - copy</br>00100000 - 32 - rename |  |
| Finalized | boolean| `bool` |  | |  |  |
| ID | string| `string` |  | |  |  |
| IsRedeemRequired | boolean| `bool` |  | |  |  |
| LastRedeemedSeq | int64 (formatted integer)| `int64` |  | |  |  |
| LatestRedeemedWM | string| `string` |  | |  |  |
| OwnerID | string| `string` |  | |  |  |
| OwnerPublicKey | string| `string` |  | |  |  |
| RepairerID | string| `string` |  | |  |  |
| StartTime | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| Terms | [][Terms](#terms)| `[]*Terms` |  | | Has many terms</br>If Preload("Terms") is required replace tag `gorm:"-"` with `gorm:"foreignKey:AllocationID"` |  |
| TimeUnit | [Duration](#duration)| `Duration` |  | |  |  |
| TotalSize | int64 (formatted integer)| `int64` |  | |  |  |
| Tx | string| `string` |  | |  |  |
| UsedSize | int64 (formatted integer)| `int64` |  | |  |  |



### <span id="auth-ticket"></span> AuthTicket


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| ActualFileHash | string| `string` |  | |  |  |
| AllocationID | string| `string` |  | |  |  |
| ClientID | string| `string` |  | |  |  |
| Encrypted | boolean| `bool` |  | |  |  |
| FileName | string| `string` |  | |  |  |
| FilePathHash | string| `string` |  | |  |  |
| OwnerID | string| `string` |  | |  |  |
| ReEncryptionKey | string| `string` |  | |  |  |
| RefType | string| `string` |  | |  |  |
| Signature | string| `string` |  | |  |  |
| expiration | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| timestamp | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |



### <span id="auth-ticket-response"></span> AuthTicketResponse


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| AuthTicket | string| `string` |  | |  |  |



### <span id="base-file-changer"></span> BaseFileChanger


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| ActualFileHashSignature | string| `string` |  | | client side: |  |
| ActualHash | string| `string` |  | | client side: |  |
| ActualSize | int64 (formatted integer)| `int64` |  | | client side: |  |
| ActualThumbnailHash | string| `string` |  | | client side: |  |
| ActualThumbnailSize | int64 (formatted integer)| `int64` |  | | client side: |  |
| AllocationID | string| `string` |  | | server side: update them by ChangeProcessor |  |
| ChunkEndIndex | int64 (formatted integer)| `int64` |  | |  |  |
| ChunkHash | string| `string` |  | |  |  |
| ChunkSize | int64 (formatted integer)| `int64` |  | |  |  |
| ChunkStartIndex | int64 (formatted integer)| `int64` |  | |  |  |
| ConnectionID | string| `string` |  | | client side: unmarshal them from 'updateMeta'/'uploadMeta' |  |
| CustomMeta | string| `string` |  | |  |  |
| EncryptedKey | string| `string` |  | |  |  |
| EncryptedKeyPoint | string| `string` |  | |  |  |
| Filename | string| `string` |  | | client side: |  |
| FixedMerkleRoot | string| `string` |  | | client side:</br>client side: |  |
| IsFinal | boolean| `bool` |  | |  |  |
| MimeType | string| `string` |  | | client side: |  |
| Path | string| `string` |  | | client side: |  |
| Size | int64 (formatted integer)| `int64` |  | |  |  |
| ThumbnailFilename | string| `string` |  | |  |  |
| ThumbnailHash | string| `string` |  | | server side: |  |
| ThumbnailSize | int64 (formatted integer)| `int64` |  | |  |  |
| UploadOffset | int64 (formatted integer)| `int64` |  | |  |  |
| ValidationRoot | string| `string` |  | | client side: |  |
| ValidationRootSignature | string| `string` |  | | client side: |  |



### <span id="challenge-timing"></span> ChallengeTiming


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| ChallengeID | string| `string` |  | | ChallengeID is the challenge ID generated on blockchain. |  |
| FileSize | int64 (formatted integer)| `int64` |  | | FileSize is size of file that was randomly selected for challenge |  |
| ProofGenTime | int64 (formatted integer)| `int64` |  | | ProofGenTime is the time taken in millisecond to generate challenge proof for the file |  |
| cancelled | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| closed | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| complete_validation | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| created_at_blobber | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| created_at_chain | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| expiration | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| txn_submission | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| txn_verification | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| updated | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |



### <span id="commit-result"></span> CommitResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| AllocationRoot | string| `string` |  | |  |  |
| ErrorMessage | string| `string` |  | |  |  |
| Success | boolean| `bool` |  | |  |  |
| write_marker | [WriteMarkerEntity](#write-marker-entity)| `WriteMarkerEntity` |  | |  |  |



### <span id="connection-result"></span> ConnectionResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| AllocationRoot | string| `string` |  | |  |  |
| ConnectionID | string| `string` |  | |  |  |



### <span id="deleted-at"></span> DeletedAt


  


* composed type [NullTime](#null-time)

### <span id="download-response"></span> DownloadResponse


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| Data | []uint8 (formatted integer)| `[]uint8` |  | |  |  |
| Success | boolean| `bool` |  | |  |  |
| latest_rm | [ReadMarker](#read-marker)| `ReadMarker` |  | |  |  |



### <span id="duration"></span> Duration


> A Duration represents the elapsed time between two instants
as an int64 nanosecond count. The representation limits the
largest representable duration to approximately 290 years.
  



| Name | Type | Go type | Default | Description | Example |
|------|------|---------| ------- |-------------|---------|
| Duration | int64 (formatted integer)| int64 | | A Duration represents the elapsed time between two instants</br>as an int64 nanosecond count. The representation limits the</br>largest representable duration to approximately 290 years. |  |



### <span id="file-download-response"></span> FileDownloadResponse


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| Data | []uint8 (formatted integer)| `[]uint8` |  | |  |  |
| Indexes | [][[]int64](#int64)| `[][]int64` |  | |  |  |
| Nodes | [][[][]uint8](#uint8)| `[][][]uint8` |  | |  |  |



### <span id="file-stats"></span> FileStats


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| CreatedAt | date-time (formatted string)| `strfmt.DateTime` |  | |  |  |
| DeletedAt | [DeletedAt](#deleted-at)| `DeletedAt` |  | |  |  |
| FailedChallenges | int64 (formatted integer)| `int64` |  | |  |  |
| LastChallengeResponseTxn | string| `string` |  | |  |  |
| NumBlockDownloads | int64 (formatted integer)| `int64` |  | |  |  |
| NumUpdates | int64 (formatted integer)| `int64` |  | |  |  |
| OnChain | boolean| `bool` |  | |  |  |
| Ref | [Ref](#ref)| `Ref` |  | |  |  |
| SuccessChallenges | int64 (formatted integer)| `int64` |  | |  |  |
| UpdatedAt | date-time (formatted string)| `strfmt.DateTime` |  | |  |  |
| WriteMarkerRedeemTxn | string| `string` |  | |  |  |



### <span id="latest-write-marker-result"></span> LatestWriteMarkerResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| Version | string| `string` |  | |  |  |
| latest_write_marker | [WriteMarker](#write-marker)| `WriteMarker` |  | |  |  |
| prev_write_marker | [WriteMarker](#write-marker)| `WriteMarker` |  | |  |  |



### <span id="list-result"></span> ListResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| AllocationRoot | string| `string` |  | |  |  |
| Entities | [][map[string]interface{}](#map-string-interface)| `[]map[string]interface{}` |  | |  |  |
| Meta | map of any | `map[string]interface{}` |  | |  |  |



### <span id="lock-result"></span> LockResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| CreatedAt | int64 (formatted integer)| `int64` |  | |  |  |
| status | [LockStatus](#lock-status)| `LockStatus` |  | |  |  |



### <span id="lock-status"></span> LockStatus


> LockStatus lock status
  



| Name | Type | Go type | Default | Description | Example |
|------|------|---------| ------- |-------------|---------|
| LockStatus | int64 (formatted integer)| int64 | | LockStatus lock status |  |



### <span id="model-with-t-s"></span> ModelWithTS


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| CreatedAt | date-time (formatted string)| `strfmt.DateTime` |  | |  |  |
| UpdatedAt | date-time (formatted string)| `strfmt.DateTime` |  | |  |  |



### <span id="null-time"></span> NullTime


> NullTime implements the [Scanner] interface so
it can be used as a scan destination, similar to [NullString].
  





**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| Time | date-time (formatted string)| `strfmt.DateTime` |  | |  |  |
| Valid | boolean| `bool` |  | |  |  |



### <span id="paginated-ref"></span> PaginatedRef


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| ActualFileHash | string| `string` |  | |  |  |
| ActualFileHashSignature | string| `string` |  | |  |  |
| ActualFileSize | int64 (formatted integer)| `int64` |  | |  |  |
| ActualThumbnailHash | string| `string` |  | |  |  |
| ActualThumbnailSize | int64 (formatted integer)| `int64` |  | |  |  |
| AllocationID | string| `string` |  | |  |  |
| AllocationRoot | string| `string` |  | |  |  |
| ChunkSize | int64 (formatted integer)| `int64` |  | |  |  |
| CustomMeta | string| `string` |  | |  |  |
| EncryptedKey | string| `string` |  | |  |  |
| EncryptedKeyPoint | string| `string` |  | |  |  |
| FileID | string| `string` |  | |  |  |
| FixedMerkleRoot | string| `string` |  | |  |  |
| Hash | string| `string` |  | |  |  |
| ID | int64 (formatted integer)| `int64` |  | |  |  |
| LookupHash | string| `string` |  | |  |  |
| MimeType | string| `string` |  | |  |  |
| Name | string| `string` |  | |  |  |
| NumBlocks | int64 (formatted integer)| `int64` |  | |  |  |
| ParentPath | string| `string` |  | |  |  |
| Path | string| `string` |  | |  |  |
| PathHash | string| `string` |  | |  |  |
| PathLevel | int64 (formatted integer)| `int64` |  | |  |  |
| Size | int64 (formatted integer)| `int64` |  | |  |  |
| ThumbnailHash | string| `string` |  | |  |  |
| ThumbnailSize | int64 (formatted integer)| `int64` |  | |  |  |
| Type | string| `string` |  | |  |  |
| ValidationRoot | string| `string` |  | |  |  |
| ValidationRootSignature | string| `string` |  | |  |  |
| created_at | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| updated_at | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |



### <span id="playlist-file"></span> PlaylistFile


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| LookupHash | string| `string` |  | |  |  |
| MimeType | string| `string` |  | |  |  |
| Name | string| `string` |  | |  |  |
| NumBlocks | int64 (formatted integer)| `int64` |  | |  |  |
| ParentPath | string| `string` |  | |  |  |
| Path | string| `string` |  | |  |  |
| Size | int64 (formatted integer)| `int64` |  | |  |  |
| Type | string| `string` |  | |  |  |



### <span id="read-marker"></span> ReadMarker


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| AllocationID | string| `string` |  | |  |  |
| BlobberID | string| `string` |  | |  |  |
| ClientID | string| `string` |  | |  |  |
| ClientPublicKey | string| `string` |  | |  |  |
| OwnerID | string| `string` |  | |  |  |
| ReadCounter | int64 (formatted integer)| `int64` |  | |  |  |
| SessionRC | int64 (formatted integer)| `int64` |  | |  |  |
| Signature | string| `string` |  | |  |  |
| timestamp | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |



### <span id="recent-ref-result"></span> RecentRefResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| Offset | int64 (formatted integer)| `int64` |  | |  |  |
| Refs | [][PaginatedRef](#paginated-ref)| `[]*PaginatedRef` |  | |  |  |



### <span id="ref"></span> Ref


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| ActualFileHash | string| `string` |  | |  |  |
| ActualFileHashSignature | string| `string` |  | |  |  |
| ActualFileSize | int64 (formatted integer)| `int64` |  | |  |  |
| ActualThumbnailHash | string| `string` |  | |  |  |
| ActualThumbnailSize | int64 (formatted integer)| `int64` |  | |  |  |
| AllocationID | string| `string` |  | |  |  |
| AllocationRoot | string| `string` |  | |  |  |
| Children | [][Ref](#ref)| `[]*Ref` |  | |  |  |
| ChunkSize | int64 (formatted integer)| `int64` |  | |  |  |
| CreatedAt | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| CustomMeta | string| `string` |  | |  |  |
| DeletedAt | [DeletedAt](#deleted-at)| `DeletedAt` |  | |  |  |
| EncryptedKey | string| `string` |  | |  |  |
| EncryptedKeyPoint | string| `string` |  | |  |  |
| FileID | string| `string` |  | |  |  |
| FileMetaHash | string| `string` |  | |  |  |
| FixedMerkleRoot | string| `string` |  | |  |  |
| Hash | string| `string` |  | |  |  |
| HashToBeComputed | boolean| `bool` |  | |  |  |
| ID | int64 (formatted integer)| `int64` |  | |  |  |
| IsPrecommit | boolean| `bool` |  | |  |  |
| LookupHash | string| `string` |  | |  |  |
| MimeType | string| `string` |  | |  |  |
| Name | string| `string` |  | |  |  |
| NumBlockDownloads | int64 (formatted integer)| `int64` |  | |  |  |
| NumBlocks | int64 (formatted integer)| `int64` |  | |  |  |
| NumUpdates | int64 (formatted integer)| `int64` |  | |  |  |
| ParentPath | string| `string` |  | |  |  |
| Path | string| `string` |  | |  |  |
| PathHash | string| `string` |  | |  |  |
| PathLevel | int64 (formatted integer)| `int64` |  | |  |  |
| PrevThumbnailHash | string| `string` |  | |  |  |
| PrevValidationRoot | string| `string` |  | |  |  |
| Size | int64 (formatted integer)| `int64` |  | |  |  |
| ThumbnailHash | string| `string` |  | |  |  |
| ThumbnailSize | int64 (formatted integer)| `int64` |  | |  |  |
| Type | string| `string` |  | |  |  |
| UpdatedAt | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |
| ValidationRoot | string| `string` |  | |  |  |
| ValidationRootSignature | string| `string` |  | |  |  |



### <span id="ref-result"></span> RefResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| OffsetPath | string| `string` |  | |  |  |
| Refs | [][PaginatedRef](#paginated-ref)| `[]*PaginatedRef` |  | |  |  |
| TotalPages | int64 (formatted integer)| `int64` |  | |  |  |
| latest_write_marker | [WriteMarker](#write-marker)| `WriteMarker` |  | |  |  |
| offset_date | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |



### <span id="reference-path"></span> ReferencePath


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| List | [][ReferencePath](#reference-path)| `[]*ReferencePath` |  | |  |  |
| Meta | map of any | `map[string]interface{}` |  | |  |  |



### <span id="reference-path-result"></span> ReferencePathResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| List | [][ReferencePath](#reference-path)| `[]*ReferencePath` |  | |  |  |
| Meta | map of any | `map[string]interface{}` |  | |  |  |
| Version | string| `string` |  | |  |  |
| latest_write_marker | [WriteMarker](#write-marker)| `WriteMarker` |  | |  |  |



### <span id="share-info"></span> ShareInfo


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| AvailableAt | date-time (formatted string)| `strfmt.DateTime` |  | |  |  |
| ClientEncryptionPublicKey | string| `string` |  | |  |  |
| ClientID | string| `string` |  | |  |  |
| ExpiryAt | date-time (formatted string)| `strfmt.DateTime` |  | |  |  |
| FilePathHash | string| `string` |  | |  |  |
| ID | int64 (formatted integer)| `int64` |  | |  |  |
| OwnerID | string| `string` |  | |  |  |
| ReEncryptionKey | string| `string` |  | |  |  |
| Revoked | boolean| `bool` |  | |  |  |



### <span id="terms"></span> Terms


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| Allocation | [Allocation](#allocation)| `Allocation` |  | |  |  |
| AllocationID | string| `string` |  | |  |  |
| BlobberID | string| `string` |  | |  |  |
| ID | int64 (formatted integer)| `int64` |  | |  |  |
| ReadPrice | uint64 (formatted integer)| `uint64` |  | |  |  |
| WritePrice | uint64 (formatted integer)| `uint64` |  | |  |  |



### <span id="timestamp"></span> Timestamp


  

| Name | Type | Go type | Default | Description | Example |
|------|------|---------| ------- |-------------|---------|
| Timestamp | int64 (formatted integer)| int64 | |  |  |



### <span id="upload-file-changer"></span> UploadFileChanger


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| ActualFileHashSignature | string| `string` |  | | client side: |  |
| ActualHash | string| `string` |  | | client side: |  |
| ActualSize | int64 (formatted integer)| `int64` |  | | client side: |  |
| ActualThumbnailHash | string| `string` |  | | client side: |  |
| ActualThumbnailSize | int64 (formatted integer)| `int64` |  | | client side: |  |
| AllocationID | string| `string` |  | | server side: update them by ChangeProcessor |  |
| ChunkEndIndex | int64 (formatted integer)| `int64` |  | |  |  |
| ChunkHash | string| `string` |  | |  |  |
| ChunkSize | int64 (formatted integer)| `int64` |  | |  |  |
| ChunkStartIndex | int64 (formatted integer)| `int64` |  | |  |  |
| ConnectionID | string| `string` |  | | client side: unmarshal them from 'updateMeta'/'uploadMeta' |  |
| CustomMeta | string| `string` |  | |  |  |
| EncryptedKey | string| `string` |  | |  |  |
| EncryptedKeyPoint | string| `string` |  | |  |  |
| Filename | string| `string` |  | | client side: |  |
| FixedMerkleRoot | string| `string` |  | | client side:</br>client side: |  |
| IsFinal | boolean| `bool` |  | |  |  |
| MimeType | string| `string` |  | | client side: |  |
| Path | string| `string` |  | | client side: |  |
| Size | int64 (formatted integer)| `int64` |  | |  |  |
| ThumbnailFilename | string| `string` |  | |  |  |
| ThumbnailHash | string| `string` |  | | server side: |  |
| ThumbnailSize | int64 (formatted integer)| `int64` |  | |  |  |
| UploadOffset | int64 (formatted integer)| `int64` |  | |  |  |
| ValidationRoot | string| `string` |  | | client side: |  |
| ValidationRootSignature | string| `string` |  | | client side: |  |



### <span id="upload-result"></span> UploadResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| Filename | string| `string` |  | |  |  |
| FixedMerkleRoot | string| `string` |  | |  |  |
| Hash | string| `string` |  | |  |  |
| Size | int64 (formatted integer)| `int64` |  | |  |  |
| UploadLength | int64 (formatted integer)| `int64` |  | | UploadLength indicates the size of the entire upload in bytes. The value MUST be a non-negative integer. |  |
| UploadOffset | int64 (formatted integer)| `int64` |  | | Upload-Offset indicates a byte offset within a resource. The value MUST be a non-negative integer. |  |
| ValidationRoot | string| `string` |  | |  |  |



### <span id="write-marker"></span> WriteMarker


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| AllocationID | string| `string` |  | |  |  |
| AllocationRoot | string| `string` |  | |  |  |
| BlobberID | string| `string` |  | |  |  |
| ChainHash | string| `string` |  | | ChainHash is the sha256 hash of the previous chain hash and the current allocation root |  |
| ChainLength | int64 (formatted integer)| `int64` |  | |  |  |
| ChainSize | int64 (formatted integer)| `int64` |  | |  |  |
| ClientID | string| `string` |  | |  |  |
| FileMetaRoot | string| `string` |  | |  |  |
| PreviousAllocationRoot | string| `string` |  | |  |  |
| Signature | string| `string` |  | |  |  |
| Size | int64 (formatted integer)| `int64` |  | |  |  |
| Version | string| `string` |  | |  |  |
| timestamp | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |



### <span id="write-marker-entity"></span> WriteMarkerEntity


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| ClientPublicKey | string| `string` |  | |  |  |
| CloseTxnID | string| `string` |  | |  |  |
| CloseTxnNonce | int64 (formatted integer)| `int64` |  | |  |  |
| ConnectionID | string| `string` |  | |  |  |
| CreatedAt | date-time (formatted string)| `strfmt.DateTime` |  | |  |  |
| Latest | boolean| `bool` |  | |  |  |
| ReedeemRetries | int64 (formatted integer)| `int64` |  | |  |  |
| Sequence | int64 (formatted integer)| `int64` |  | |  |  |
| Status | [WriteMarkerStatus](#write-marker-status)| `WriteMarkerStatus` |  | |  |  |
| StatusMessage | string| `string` |  | |  |  |
| UpdatedAt | date-time (formatted string)| `strfmt.DateTime` |  | |  |  |
| WM | [WriteMarker](#write-marker)| `WriteMarker` |  | |  |  |



### <span id="write-marker-status"></span> WriteMarkerStatus


  

| Name | Type | Go type | Default | Description | Example |
|------|------|---------| ------- |-------------|---------|
| WriteMarkerStatus | int64 (formatted integer)| int64 | |  |  |


