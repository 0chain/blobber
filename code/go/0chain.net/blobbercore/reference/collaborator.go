package reference

import (
	"context"
	"time"

	"0chain.net/blobbercore/datastore"
)

type Collaborator struct {
	RefID     int64     `gorm:"ref_id" json:"ref_id"`
	ClientID  string    `gorm:"client_id" json:"client_id"`
	CreatedAt time.Time `gorm:"created_at" json:"created_at"`
}

func (Collaborator) TableName() string {
	return "collaborators"
}

func AddCollaborator(ctx context.Context, refID int64, clientID string) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Create(&Collaborator{
		RefID:    refID,
		ClientID: clientID,
	}).Error
}

func GetCollaborators(ctx context.Context, refID int64) ([]Collaborator, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	collaborators := []Collaborator{}
	err := db.Table((&Collaborator{}).TableName()).
		Where(&Collaborator{RefID: refID}).
		Order("created_at desc").
		Find(&collaborators).Error
	return collaborators, err
}

func IsACollaborator(ctx context.Context, refID int64, clientID string) bool {
	db := datastore.GetStore().GetTransaction(ctx)
	var collaboratorCount int64
	err := db.Table((&Collaborator{}).TableName()).
		Where(&Collaborator{
			RefID:    refID,
			ClientID: clientID,
		}).
		Count(&collaboratorCount).Error
	if err != nil {
		return false
	}
	return collaboratorCount > 0
}
