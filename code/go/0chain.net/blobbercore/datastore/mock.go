package datastore

import (
	"context"
	"database/sql"
	"gorm.io/gorm/clause"
)

func MockTheStore(mock Store) {
	TheStore = mock
}

type MockStore struct{}

func (store *MockStore) Open() error {
	return nil
}

func (store *MockStore) Close() {
}

func (store *MockStore) CreateTransaction(ctx context.Context) context.Context {
	return ctx
}

func (store *MockStore) GetTransaction(ctx context.Context) Transaction {
	return &MockTransaction{}
}

type MockTransaction struct{}

func (t *MockTransaction) Model(value interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Table(name string, args ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Select(query interface{}, args ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Where(query interface{}, args ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Not(query interface{}, args ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Or(query interface{}, args ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Joins(query string, args ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Group(name string) (tx Transaction) {
	return t
}
func (t *MockTransaction) Having(query interface{}, args ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Order(value interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Create(value interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Save(value interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) First(dest interface{}, conds ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Begin(opts ...*sql.TxOptions) Transaction {
	return t
}
func (t *MockTransaction) Commit() Transaction {
	return t
}
func (t *MockTransaction) Rollback() Transaction {
	return t
}
func (t *MockTransaction) Find(dest interface{}, conds ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Limit(limit int) (tx Transaction) {
	return t
}
func (t *MockTransaction) Rows() (*sql.Rows, error) {
	return nil, nil
}
func (t *MockTransaction) Scan(dest interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Error() error {
	return nil
}
func (t *MockTransaction) Delete(value interface{}, conds ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Take(dest interface{}, conds ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Raw(sql string, values ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Update(column string, value interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Updates(values interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Count(count *int64) (tx Transaction) {
	return t
}
func (t *MockTransaction) Row() *sql.Row {
	return nil
}
func (t *MockTransaction) Preload(query string, args ...interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Offset(offset int) (tx Transaction) {
	return t
}
func (t *MockTransaction) Pluck(column string, dest interface{}) (tx Transaction) {
	return t
}
func (t *MockTransaction) Clauses(conds ...clause.Expression) (tx Transaction) {
	return t
}
