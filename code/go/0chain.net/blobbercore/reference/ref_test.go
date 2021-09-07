package reference

import (
	"context"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"reflect"
	"testing"
	"time"
)

func TestRef_GetListingData(t *testing.T) {
	type fields struct {
		ID                  int64
		Type                string
		AllocationID        string
		LookupHash          string
		Name                string
		Path                string
		Hash                string
		NumBlocks           int64
		PathHash            string
		ParentPath          string
		PathLevel           int
		CustomMeta          string
		ContentHash         string
		Size                int64
		MerkleRoot          string
		ActualFileSize      int64
		ActualFileHash      string
		MimeType            string
		WriteMarker         string
		ThumbnailSize       int64
		ThumbnailHash       string
		ActualThumbnailSize int64
		ActualThumbnailHash string
		EncryptedKey        string
		Attributes          datatypes.JSON
		Children            []*Ref
		childrenLoaded      bool
		OnCloud             bool
		CommitMetaTxns      []CommitMetaTxn
		CreatedAt           time.Time
		UpdatedAt           time.Time
		DeletedAt           gorm.DeletedAt
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   map[string]interface{}
	}{
		{
			name: "success",
			fields: fields{
				ID:                  23,
				Type:                "f",
				AllocationID:        "321323",
				LookupHash:          "32323",
				Name:                "32323",
				Path:                "3232",
				Hash:                "32323",
				NumBlocks:           2,
				PathHash:            "fefef",
				ParentPath:          "fefe",
				PathLevel:           2,
				CustomMeta:          "fefe",
				ContentHash:         "acdscds",
				Size:                1,
				MerkleRoot:          "cdcdas",
				ActualFileSize:      1,
				ActualFileHash:      "cwedwed",
				MimeType:            "cdscasdc",
				WriteMarker:         "cdacas",
				ThumbnailSize:       2,
				ThumbnailHash:       "ewfewfw",
				ActualThumbnailSize: 2,
				ActualThumbnailHash: "cdcew",
				EncryptedKey:        "dwdewd",
				Attributes:          nil,
				Children:            nil,
				childrenLoaded:      true,
				OnCloud:             true,
				CommitMetaTxns:      []CommitMetaTxn{
					{
						RefID:     323232,
						TxnID:     "sswswsw",
						CreatedAt: time.Now(),
					},
				},
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
				DeletedAt:           gorm.DeletedAt{
					Time: time.Now(),
					Valid: true,
				},
			},
			args: args{
				ctx: context.Background(),
			},
			want: map[string]interface{}{
				"name" : "32323",
				"content_hash": "acdscds",
				"thumbnail_size": int64(2),
				"actual_thumbnail_hash": "cdcew",
				"updated_at": time.Now(),
				"created_at": time.Now(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Ref{
				ID:                  tt.fields.ID,
				Type:                tt.fields.Type,
				AllocationID:        tt.fields.AllocationID,
				LookupHash:          tt.fields.LookupHash,
				Name:                tt.fields.Name,
				Path:                tt.fields.Path,
				Hash:                tt.fields.Hash,
				NumBlocks:           tt.fields.NumBlocks,
				PathHash:            tt.fields.PathHash,
				ParentPath:          tt.fields.ParentPath,
				PathLevel:           tt.fields.PathLevel,
				CustomMeta:          tt.fields.CustomMeta,
				ContentHash:         tt.fields.ContentHash,
				Size:                tt.fields.Size,
				MerkleRoot:          tt.fields.MerkleRoot,
				ActualFileSize:      tt.fields.ActualFileSize,
				ActualFileHash:      tt.fields.ActualFileHash,
				MimeType:            tt.fields.MimeType,
				WriteMarker:         tt.fields.WriteMarker,
				ThumbnailSize:       tt.fields.ThumbnailSize,
				ThumbnailHash:       tt.fields.ThumbnailHash,
				ActualThumbnailSize: tt.fields.ActualThumbnailSize,
				ActualThumbnailHash: tt.fields.ActualThumbnailHash,
				EncryptedKey:        tt.fields.EncryptedKey,
				Attributes:          tt.fields.Attributes,
				Children:            tt.fields.Children,
				childrenLoaded:      tt.fields.childrenLoaded,
				OnCloud:             tt.fields.OnCloud,
				CommitMetaTxns:      tt.fields.CommitMetaTxns,
				CreatedAt:           tt.fields.CreatedAt,
				UpdatedAt:           tt.fields.UpdatedAt,
				DeletedAt:           tt.fields.DeletedAt,
			}
			if got := r.GetListingData(tt.args.ctx); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetListingData() = %v, want %v", got, tt.want)
			}
		})
	}
}


func TestListingDataToRef(t *testing.T) {
	type args struct {
		refMap map[string]interface{}
	}
	tests := []struct {
		name string
		args args
		want *Ref
	}{
		{
			name: "success",
			args: args{
				refMap: map[string]interface{}{
					"name" : "32323",
					"content_hash": "acdscds",
					"thumbnail_size": int64(2),
					"actual_thumbnail_hash": "cdcew",
					"updated_at": "979997997",
					"created_at": "979997997",
				},
			},
			want: &Ref{
				Name: "sasa",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}
		for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ListingDataToRef(tt.args.refMap); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListingDataToRef() = %v, want %v", got, tt.want)
			}
		})
	}
}