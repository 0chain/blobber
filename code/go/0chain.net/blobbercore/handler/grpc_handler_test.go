//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"context"
	"errors"
	"net"
	"regexp"
	"testing"
	"time"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/gorm"
)

var (
	lis *bufconn.Listener
)

func startGRPCServer(t *testing.T) {
	lis = bufconn.Listen(1024 * 1024)
	grpcS := NewGRPCServerWithMiddlewares(mux.NewRouter())
	go func() {
		if err := grpcS.Serve(lis); err != nil {
			t.Errorf("Server exited with error: %v", err)
		}
	}()
}

func makeTestClient() (blobbergrpc.BlobberServiceClient, *grpc.ClientConn, error) {
	var (
		ctx       = context.Background()
		bufDialer = func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}
	)
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}

	return blobbergrpc.NewBlobberServiceClient(conn), conn, err
}

func makeTestAllocation(exp common.Timestamp) *allocation.Allocation {
	allocID := "allocation_id"
	alloc := allocation.Allocation{
		Tx: "allocation_id",
		ID: allocID,
		Terms: []*allocation.Terms{
			{
				ID:           1,
				AllocationID: allocID,
			},
		},
		Expiration:  exp,
		FileOptions: 63, // 0011 1111 (All Allowed)
	}
	return &alloc
}

func Test_GetAllocation(t *testing.T) {
	setup(t)

	startGRPCServer(t)

	grpcCl, conn, err := makeTestClient()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ts := time.Now().Add(time.Hour)
	alloc := makeTestAllocation(common.Timestamp(ts.Unix()))

	expiredAlloc := makeTestAllocation(0)

	type (
		args struct {
			allocationR *blobbergrpc.GetAllocationRequest
		}

		test struct {
			name         string
			mockSetup    func(sqlmock.Sqlmock)
			args         args
			wantCode     string
			wantAlloc    *blobbergrpc.Allocation
			expectCommit bool
		}
	)
	tests := []test{
		{
			name: "OK",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(
					regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration),
					)

				mock.ExpectQuery(
					regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)
			},
			args: args{
				allocationR: &blobbergrpc.GetAllocationRequest{
					Id: alloc.Tx,
				},
			},
			expectCommit: true,
			wantCode:     codes.OK.String(),
			wantAlloc:    convert.AllocationToGRPCAllocation(alloc),
		},
		{
			name: "Committing_Transaction_ERR",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(
					regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration),
					)

				mock.ExpectQuery(
					regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)
			},
			args: args{
				allocationR: &blobbergrpc.GetAllocationRequest{
					Id: alloc.Tx,
				},
			},
			expectCommit: true,
			wantCode:     codes.OK.String(),
			wantAlloc:    convert.AllocationToGRPCAllocation(alloc),
		},
		{
			name: "Expired_Allocation_ERR",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(
					regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(expiredAlloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date"}).
							AddRow(expiredAlloc.ID, expiredAlloc.Tx, expiredAlloc.Expiration),
					)

				mock.ExpectQuery(
					regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(expiredAlloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(expiredAlloc.Terms[0].ID, expiredAlloc.Terms[0].AllocationID),
					)
			},
			args: args{
				allocationR: &blobbergrpc.GetAllocationRequest{
					Id: expiredAlloc.Tx,
				},
			},
			wantCode: codes.Unknown.String(),
		},
		{
			name: "Empty_ID_ERR",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(
					regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs("").
					WillReturnError(gorm.ErrRecordNotFound)
			},
			args: args{
				allocationR: &blobbergrpc.GetAllocationRequest{
					Id: "",
				},
			},
			wantCode: codes.Unknown.String(),
		},
		{
			name: "Unexpected_DB_ERR",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(
					regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs("id").
					WillReturnError(errors.New(""))
			},
			args: args{
				allocationR: &blobbergrpc.GetAllocationRequest{
					Id: "id",
				},
			},
			wantCode: codes.Unknown.String(),
		},
	}
	for _, test := range tests {
		t.Run(test.name,
			func(t *testing.T) {
				var mock = datastore.MockTheStore(t)
				test.mockSetup(mock)
				if test.expectCommit {
					mock.ExpectCommit()
				}

				resp, err := grpcCl.GetAllocation(context.TODO(), test.args.allocationR)
				assert.Equal(t, test.wantCode, status.Code(err).String())
				if err == nil {
					assert.Equal(t, test.wantAlloc, resp.Allocation)
				}
			},
		)
	}
}
