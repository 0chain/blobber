package datastore

import (
	"context"
	"database/sql"
	gorm "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func CreateTransaction(ctx context.Context) context.Context {
	return TheStore.CreateTransaction(ctx)
}

func GetTransaction(ctx context.Context) Transaction {
	return TheStore.GetTransaction(ctx)
}

type Transaction interface {
	Model(value interface{}) (tx Transaction)
	Table(name string, args ...interface{}) (tx Transaction)
	Select(query interface{}, args ...interface{}) (tx Transaction)
	Where(query interface{}, args ...interface{}) (tx Transaction)
	Not(query interface{}, args ...interface{}) (tx Transaction)
	Or(query interface{}, args ...interface{}) (tx Transaction)
	Joins(query string, args ...interface{}) (tx Transaction)
	Group(name string) (tx Transaction)
	Having(query interface{}, args ...interface{}) (tx Transaction)
	Order(value interface{}) (tx Transaction)
	Create(value interface{}) (tx Transaction)
	Save(value interface{}) (tx Transaction)
	First(dest interface{}, conds ...interface{}) (tx Transaction)
	Begin(opts ...*sql.TxOptions) Transaction
	Commit() Transaction
	Rollback() Transaction
	Find(dest interface{}, conds ...interface{}) (tx Transaction)
	Limit(limit int) (tx Transaction)
	Rows() (*sql.Rows, error)
	Scan(dest interface{}) (tx Transaction)
	Error() error
	Delete(value interface{}, conds ...interface{}) (tx Transaction)
	Take(dest interface{}, conds ...interface{}) (tx Transaction)
	Raw(sql string, values ...interface{}) (tx Transaction)
	Update(column string, value interface{}) (tx Transaction)
	Updates(values interface{}) (tx Transaction)
	Count(count *int64) (tx Transaction)
	Row() *sql.Row
	Preload(query string, args ...interface{}) (tx Transaction)
	Offset(offset int) (tx Transaction)
	Pluck(column string, dest interface{}) (tx Transaction)
	Clauses(conds ...clause.Expression) (tx Transaction)
}

type transaction struct {
	*gorm.DB
}

func (t *transaction) Model(value interface{}) (tx Transaction) {
	return &transaction{t.DB.Model(value)}
}
func (t *transaction) Table(name string, args ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Table(name, args...)}
}
func (t *transaction) Select(query interface{}, args ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Select(query, args...)}
}
func (t *transaction) Where(query interface{}, args ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Where(query, args...)}
}
func (t *transaction) Not(query interface{}, args ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Not(query, args...)}
}
func (t *transaction) Or(query interface{}, args ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Or(query, args...)}
}
func (t *transaction) Joins(query string, args ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Joins(query, args...)}
}
func (t *transaction) Group(name string) (tx Transaction) {
	return &transaction{t.DB.Group(name)}
}
func (t *transaction) Having(query interface{}, args ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Having(query, args...)}
}
func (t *transaction) Order(value interface{}) (tx Transaction) {
	return &transaction{t.DB.Order(value)}
}
func (t *transaction) Create(value interface{}) (tx Transaction) {
	return &transaction{t.DB.Create(value)}
}
func (t *transaction) Save(value interface{}) (tx Transaction) {
	return &transaction{t.DB.Save(value)}
}
func (t *transaction) First(dest interface{}, conds ...interface{}) (tx Transaction) {
	return &transaction{t.DB.First(dest, conds...)}
}
func (t *transaction) Begin(opts ...*sql.TxOptions) Transaction {
	return &transaction{t.DB.Begin(opts...)}
}
func (t *transaction) Commit() Transaction {
	return &transaction{t.DB.Commit()}
}
func (t *transaction) Rollback() Transaction {
	return &transaction{t.DB.Rollback()}
}
func (t *transaction) Find(dest interface{}, conds ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Find(dest, conds...)}
}
func (t *transaction) Limit(limit int) (tx Transaction) {
	return &transaction{t.DB.Limit(limit)}
}
func (t *transaction) Rows() (*sql.Rows, error) {
	return t.DB.Rows()
}
func (t *transaction) Scan(dest interface{}) (tx Transaction) {
	return &transaction{t.DB.Scan(dest)}
}
func (t *transaction) Error() error {
	return t.DB.Error
}
func (t *transaction) Delete(value interface{}, conds ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Delete(value, conds...)}
}
func (t *transaction) Take(dest interface{}, conds ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Take(dest, conds...)}
}
func (t *transaction) Raw(sql string, values ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Raw(sql, values...)}
}
func (t *transaction) Update(column string, value interface{}) (tx Transaction) {
	return &transaction{t.DB.Update(column, value)}
}
func (t *transaction) Updates(values interface{}) (tx Transaction) {
	return &transaction{t.DB.Updates(values)}
}
func (t *transaction) Count(count *int64) (tx Transaction) {
	return &transaction{t.DB.Count(count)}
}
func (t *transaction) Row() *sql.Row {
	return t.DB.Row()
}
func (t *transaction) Preload(query string, args ...interface{}) (tx Transaction) {
	return &transaction{t.DB.Preload(query, args...)}
}
func (t *transaction) Offset(offset int) (tx Transaction) {
	return &transaction{t.DB.Offset(offset)}
}
func (t *transaction) Pluck(column string, dest interface{}) (tx Transaction) {
	return &transaction{t.DB.Pluck(column, dest)}
}
func (t *transaction) Clauses(conds ...clause.Expression) (tx Transaction) {
	return &transaction{t.DB.Clauses(conds...)}
}
