package datastore

import "time"

type ModelWithTS struct {
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp without time zone;not null;default:now()"  json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamp without time zone;not null;default:now()"  json:"updated_at"`
}
