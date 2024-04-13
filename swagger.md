


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

### Produces
  * application/json

## All endpoints

###  operations

| Method  | URI     | Name   | Summary |
|---------|---------|--------|---------|
| GET | /allocation | [allocation](#allocation) |  |
| GET | /v1/connection/commit/{allocation} | [commithandler](#commithandler) |  |
| POST | /v1/file/commitmetatxn/{allocation} | [commitmetatxn](#commitmetatxn) |  |
| GET | /v1/connection/create/{allocation} | [connection handler](#connection-handler) |  |
| GET | /v1/file/copy/{allocation} | [copyallocation](#copyallocation) |  |
| GET | /v1/dir/{allocation} | [createdirhandler](#createdirhandler) |  |
| POST | /v1/file/download/{allocation} | [download file](#download-file) | DownloadHandler is the handler to respond to download requests from clients. |
| GET | /v1/file/list/{allocation} | [list](#list) |  |
| GET | /v1/file/move/{allocation} | [moveallocation](#moveallocation) |  |
| GET | /v1/file/refs/recent/{allocation} | [recentalloc](#recentalloc) |  |
| GET | /v1/file/objecttree/{allocation} | [referencepath](#referencepath) |  |
| GET | /v1/file/refs/{allocation} | [refshandler](#refshandler) |  |
| GET | /v1/file/rename/{allocation} | [renameallocation](#renameallocation) |  |
  


## Paths

### <span id="allocation"></span> allocation (*allocation*)

```
GET /allocation
```

get allocation details

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| id | `query` | string | `string` |  | ✓ |  | allocation ID |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#allocation-200) | OK | CommitResult |  | [schema](#allocation-200-schema) |
| [400](#allocation-400) | Bad Request |  |  | [schema](#allocation-400-schema) |
| [500](#allocation-500) | Internal Server Error |  |  | [schema](#allocation-500-schema) |

#### Responses


##### <span id="allocation-200"></span> 200 - CommitResult
Status: OK

###### <span id="allocation-200-schema"></span> Schema
   
  

[CommitResult](#commit-result)

##### <span id="allocation-400"></span> 400
Status: Bad Request

###### <span id="allocation-400-schema"></span> Schema

##### <span id="allocation-500"></span> 500
Status: Internal Server Error

###### <span id="allocation-500-schema"></span> Schema

### <span id="commithandler"></span> commithandler (*commithandler*)

```
GET /v1/connection/commit/{allocation}
```

CommitHandler is the handler to respond to upload requests from clients

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#commithandler-200) | OK | CommitResult |  | [schema](#commithandler-200-schema) |
| [400](#commithandler-400) | Bad Request |  |  | [schema](#commithandler-400-schema) |
| [500](#commithandler-500) | Internal Server Error |  |  | [schema](#commithandler-500-schema) |

#### Responses


##### <span id="commithandler-200"></span> 200 - CommitResult
Status: OK

###### <span id="commithandler-200-schema"></span> Schema
   
  

[CommitResult](#commit-result)

##### <span id="commithandler-400"></span> 400
Status: Bad Request

###### <span id="commithandler-400-schema"></span> Schema

##### <span id="commithandler-500"></span> 500
Status: Internal Server Error

###### <span id="commithandler-500-schema"></span> Schema

### <span id="commitmetatxn"></span> commitmetatxn (*commitmetatxn*)

```
POST /v1/file/commitmetatxn/{allocation}
```

CommitHandler is the handler to respond to upload requests from clients

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |
| body | `body` | string | `string` | | ✓ | | transaction id |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#commitmetatxn-200) | OK |  |  | [schema](#commitmetatxn-200-schema) |
| [400](#commitmetatxn-400) | Bad Request |  |  | [schema](#commitmetatxn-400-schema) |
| [500](#commitmetatxn-500) | Internal Server Error |  |  | [schema](#commitmetatxn-500-schema) |

#### Responses


##### <span id="commitmetatxn-200"></span> 200
Status: OK

###### <span id="commitmetatxn-200-schema"></span> Schema

##### <span id="commitmetatxn-400"></span> 400
Status: Bad Request

###### <span id="commitmetatxn-400-schema"></span> Schema

##### <span id="commitmetatxn-500"></span> 500
Status: Internal Server Error

###### <span id="commitmetatxn-500-schema"></span> Schema

### <span id="connection-handler"></span> connection handler (*connectionHandler*)

```
GET /v1/connection/create/{allocation}
```

connectionHandler is the handler to respond to create connection requests from clients

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#connection-handler-200) | OK |  |  | [schema](#connection-handler-200-schema) |
| [400](#connection-handler-400) | Bad Request |  |  | [schema](#connection-handler-400-schema) |
| [500](#connection-handler-500) | Internal Server Error |  |  | [schema](#connection-handler-500-schema) |

#### Responses


##### <span id="connection-handler-200"></span> 200
Status: OK

###### <span id="connection-handler-200-schema"></span> Schema

##### <span id="connection-handler-400"></span> 400
Status: Bad Request

###### <span id="connection-handler-400-schema"></span> Schema

##### <span id="connection-handler-500"></span> 500
Status: Internal Server Error

###### <span id="connection-handler-500-schema"></span> Schema

### <span id="copyallocation"></span> copyallocation (*copyallocation*)

```
GET /v1/file/copy/{allocation}
```

copy an allocation

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#copyallocation-200) | OK | UploadResult |  | [schema](#copyallocation-200-schema) |
| [400](#copyallocation-400) | Bad Request |  |  | [schema](#copyallocation-400-schema) |
| [500](#copyallocation-500) | Internal Server Error |  |  | [schema](#copyallocation-500-schema) |

#### Responses


##### <span id="copyallocation-200"></span> 200 - UploadResult
Status: OK

###### <span id="copyallocation-200-schema"></span> Schema
   
  

[UploadResult](#upload-result)

##### <span id="copyallocation-400"></span> 400
Status: Bad Request

###### <span id="copyallocation-400-schema"></span> Schema

##### <span id="copyallocation-500"></span> 500
Status: Internal Server Error

###### <span id="copyallocation-500-schema"></span> Schema

### <span id="createdirhandler"></span> createdirhandler (*createdirhandler*)

```
GET /v1/dir/{allocation}
```

CreateDirHandler is the handler to respond to create dir for allocation

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#createdirhandler-200) | OK | UploadResult |  | [schema](#createdirhandler-200-schema) |
| [400](#createdirhandler-400) | Bad Request |  |  | [schema](#createdirhandler-400-schema) |
| [500](#createdirhandler-500) | Internal Server Error |  |  | [schema](#createdirhandler-500-schema) |

#### Responses


##### <span id="createdirhandler-200"></span> 200 - UploadResult
Status: OK

###### <span id="createdirhandler-200-schema"></span> Schema
   
  

[UploadResult](#upload-result)

##### <span id="createdirhandler-400"></span> 400
Status: Bad Request

###### <span id="createdirhandler-400-schema"></span> Schema

##### <span id="createdirhandler-500"></span> 500
Status: Internal Server Error

###### <span id="createdirhandler-500-schema"></span> Schema

### <span id="download-file"></span> DownloadHandler is the handler to respond to download requests from clients. (*downloadFile*)

```
POST /v1/file/download/{allocation}
```

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
| X-Verify-Download | `header` | string | `string` |  |  |  | If set to "true", the download should be verified. If the mode is "thumbnail", |
| X-Version | `header` | string | `string` |  |  |  | If its value is "v2" then both allocation_id and blobber url base are hashed and verified using X-App-Client-Signature-V2. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#download-file-200) | OK | | []byte |  | [schema](#download-file-200-schema) |
| [400](#download-file-400) | Bad Request |  |  | [schema](#download-file-400-schema) |

#### Responses


##### <span id="download-file-200"></span> 200 - | []byte
Status: OK

###### <span id="download-file-200-schema"></span> Schema
   
  

[FileDownloadResponse](#file-download-response)

##### <span id="download-file-400"></span> 400
Status: Bad Request

###### <span id="download-file-400-schema"></span> Schema

### <span id="list"></span> list (*list*)

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
| auth_token | `query` | string | `string` |  |  |  | The auth ticket for the file to download if the client does not own it. Check File Sharing docs for more info. |
| limit | `query` | integer | `int64` |  | ✓ |  | The number of files to return (for pagination). |
| list | `query` | boolean | `bool` |  |  |  | Whether or not to list the files inside the directory, not just data about the path itself. |
| offset | `query` | integer | `int64` |  | ✓ |  | The number of files to skip before returning (for pagination). |
| path | `query` | string | `string` |  | ✓ |  | The path needed to list info about |
| path_hash | `query` | string | `string` |  |  |  | Lookuphash of the path needed to list info about, which is a hex hash of the path concatenated with the allocation ID. |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#list-200) | OK | ListResult |  | [schema](#list-200-schema) |
| [400](#list-400) | Bad Request |  |  | [schema](#list-400-schema) |

#### Responses


##### <span id="list-200"></span> 200 - ListResult
Status: OK

###### <span id="list-200-schema"></span> Schema
   
  

[ListResult](#list-result)

##### <span id="list-400"></span> 400
Status: Bad Request

###### <span id="list-400-schema"></span> Schema

### <span id="moveallocation"></span> moveallocation (*moveallocation*)

```
GET /v1/file/move/{allocation}
```

move an allocation

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#moveallocation-200) | OK | UploadResult |  | [schema](#moveallocation-200-schema) |
| [400](#moveallocation-400) | Bad Request |  |  | [schema](#moveallocation-400-schema) |
| [500](#moveallocation-500) | Internal Server Error |  |  | [schema](#moveallocation-500-schema) |

#### Responses


##### <span id="moveallocation-200"></span> 200 - UploadResult
Status: OK

###### <span id="moveallocation-200-schema"></span> Schema
   
  

[UploadResult](#upload-result)

##### <span id="moveallocation-400"></span> 400
Status: Bad Request

###### <span id="moveallocation-400-schema"></span> Schema

##### <span id="moveallocation-500"></span> 500
Status: Internal Server Error

###### <span id="moveallocation-500-schema"></span> Schema

### <span id="recentalloc"></span> recentalloc (*recentalloc*)

```
GET /v1/file/refs/recent/{allocation}
```

get recent allocation

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | allocation ID |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#recentalloc-200) | OK | RecentRefResult |  | [schema](#recentalloc-200-schema) |
| [400](#recentalloc-400) | Bad Request |  |  | [schema](#recentalloc-400-schema) |
| [500](#recentalloc-500) | Internal Server Error |  |  | [schema](#recentalloc-500-schema) |

#### Responses


##### <span id="recentalloc-200"></span> 200 - RecentRefResult
Status: OK

###### <span id="recentalloc-200-schema"></span> Schema
   
  

[RecentRefResult](#recent-ref-result)

##### <span id="recentalloc-400"></span> 400
Status: Bad Request

###### <span id="recentalloc-400-schema"></span> Schema

##### <span id="recentalloc-500"></span> 500
Status: Internal Server Error

###### <span id="recentalloc-500-schema"></span> Schema

### <span id="referencepath"></span> referencepath (*referencepath*)

```
GET /v1/file/objecttree/{allocation}
```

get object tree reference path

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | allocation ID |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#referencepath-200) | OK | ReferencePathResult |  | [schema](#referencepath-200-schema) |
| [400](#referencepath-400) | Bad Request |  |  | [schema](#referencepath-400-schema) |
| [500](#referencepath-500) | Internal Server Error |  |  | [schema](#referencepath-500-schema) |

#### Responses


##### <span id="referencepath-200"></span> 200 - ReferencePathResult
Status: OK

###### <span id="referencepath-200-schema"></span> Schema
   
  

[ReferencePathResult](#reference-path-result)

##### <span id="referencepath-400"></span> 400
Status: Bad Request

###### <span id="referencepath-400-schema"></span> Schema

##### <span id="referencepath-500"></span> 500
Status: Internal Server Error

###### <span id="referencepath-500-schema"></span> Schema

### <span id="refshandler"></span> refshandler (*refshandler*)

```
GET /v1/file/refs/{allocation}
```

get object tree reference path

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | allocation ID |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#refshandler-200) | OK | RefResult |  | [schema](#refshandler-200-schema) |
| [400](#refshandler-400) | Bad Request |  |  | [schema](#refshandler-400-schema) |
| [500](#refshandler-500) | Internal Server Error |  |  | [schema](#refshandler-500-schema) |

#### Responses


##### <span id="refshandler-200"></span> 200 - RefResult
Status: OK

###### <span id="refshandler-200-schema"></span> Schema
   
  

[RefResult](#ref-result)

##### <span id="refshandler-400"></span> 400
Status: Bad Request

###### <span id="refshandler-400-schema"></span> Schema

##### <span id="refshandler-500"></span> 500
Status: Internal Server Error

###### <span id="refshandler-500-schema"></span> Schema

### <span id="renameallocation"></span> renameallocation (*renameallocation*)

```
GET /v1/file/rename/{allocation}
```

rename an allocation

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | the allocation ID |

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#renameallocation-200) | OK | UploadResult |  | [schema](#renameallocation-200-schema) |
| [400](#renameallocation-400) | Bad Request |  |  | [schema](#renameallocation-400-schema) |
| [500](#renameallocation-500) | Internal Server Error |  |  | [schema](#renameallocation-500-schema) |

#### Responses


##### <span id="renameallocation-200"></span> 200 - UploadResult
Status: OK

###### <span id="renameallocation-200-schema"></span> Schema
   
  

[UploadResult](#upload-result)

##### <span id="renameallocation-400"></span> 400
Status: Bad Request

###### <span id="renameallocation-400-schema"></span> Schema

##### <span id="renameallocation-500"></span> 500
Status: Internal Server Error

###### <span id="renameallocation-500-schema"></span> Schema

## Models

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



### <span id="commit-result"></span> CommitResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| AllocationRoot | string| `string` |  | |  |  |
| ErrorMessage | string| `string` |  | |  |  |
| Success | boolean| `bool` |  | |  |  |
| write_marker | [WriteMarker](#write-marker)| `WriteMarker` |  | |  |  |



### <span id="connection-result"></span> ConnectionResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| AllocationRoot | string| `string` |  | |  |  |
| ConnectionID | string| `string` |  | |  |  |



### <span id="deleted-at"></span> DeletedAt


  


* composed type [NullTime](#null-time)

### <span id="file-download-response"></span> FileDownloadResponse


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| Data | []uint8 (formatted integer)| `[]uint8` |  | |  |  |
| Indexes | [][[]int64](#int64)| `[][]int64` |  | |  |  |
| Nodes | [][[][]uint8](#uint8)| `[][][]uint8` |  | |  |  |



### <span id="list-result"></span> ListResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| AllocationRoot | string| `string` |  | |  |  |
| Entities | [][map[string]interface{}](#map-string-interface)| `[]map[string]interface{}` |  | |  |  |
| Meta | map of any | `map[string]interface{}` |  | |  |  |



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
| Ref | [Ref](#ref)| `Ref` |  | |  |  |



### <span id="reference-path-result"></span> ReferencePathResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| List | [][ReferencePath](#reference-path)| `[]*ReferencePath` |  | |  |  |
| Meta | map of any | `map[string]interface{}` |  | |  |  |
| Ref | [Ref](#ref)| `Ref` |  | |  |  |
| latest_write_marker | [WriteMarker](#write-marker)| `WriteMarker` |  | |  |  |



### <span id="timestamp"></span> Timestamp


  

| Name | Type | Go type | Default | Description | Example |
|------|------|---------| ------- |-------------|---------|
| Timestamp | int64 (formatted integer)| int64 | |  |  |



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
| ClientID | string| `string` |  | |  |  |
| FileMetaRoot | string| `string` |  | |  |  |
| PreviousAllocationRoot | string| `string` |  | |  |  |
| Signature | string| `string` |  | |  |  |
| Size | int64 (formatted integer)| `int64` |  | |  |  |
| timestamp | [Timestamp](#timestamp)| `Timestamp` |  | |  |  |


