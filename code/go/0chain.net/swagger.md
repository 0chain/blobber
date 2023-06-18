


# 0chain Api:
  

## Informations

### Version

0.0.1

## Content negotiation


### URI Schemes
  * http
  * https

### Consumes
  * application/json

### Produces
  * application/json

## All endpoints

###  operations

| Method  | URI     | Name   | Summary |
|---------|---------|--------|---------|
| GET | /v1/block/magic/get | [getmagicblock](#getmagicblock) |  |
| GET | /v1/file/rename/{allocation} | [renameallocation](#renameallocation) |  |
  


## Paths

### <span id="getmagicblock"></span> getmagicblock (*getmagicblock*)

```
GET /v1/block/magic/get
```

a handler to respond to block queries

#### All responses
| Code | Status | Description | Has headers | Schema |
|------|--------|-------------|:-----------:|--------|
| [200](#getmagicblock-200) | OK |  |  | [schema](#getmagicblock-200-schema) |
| [404](#getmagicblock-404) | Not Found |  |  | [schema](#getmagicblock-404-schema) |

#### Responses


##### <span id="getmagicblock-200"></span> 200
Status: OK

###### <span id="getmagicblock-200-schema"></span> Schema

##### <span id="getmagicblock-404"></span> 404
Status: Not Found

###### <span id="getmagicblock-404-schema"></span> Schema

### <span id="renameallocation"></span> renameallocation (*renameallocation*)

```
GET /v1/file/rename/{allocation}
```

rename an allocation

#### Parameters

| Name | Source | Type | Go type | Separator | Required | Default | Description |
|------|--------|------|---------|-----------| :------: |---------|-------------|
| allocation | `path` | string | `string` |  | ✓ |  | offset |

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

### <span id="upload-result"></span> UploadResult


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| Filename | string| `string` |  | |  |  |
| Hash | string| `string` |  | |  |  |
| MerkleRoot | string| `string` |  | |  |  |
| Size | int64 (formatted integer)| `int64` |  | |  |  |
| UploadLength | int64 (formatted integer)| `int64` |  | | UploadLength indicates the size of the entire upload in bytes. The value MUST be a non-negative integer. |  |
| UploadOffset | int64 (formatted integer)| `int64` |  | | Upload-Offset indicates a byte offset within a resource. The value MUST be a non-negative integer. |  |


