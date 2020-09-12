package readmarker

import (
	"context"
	"encoding/json"
	"time"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/constants"

	"0chain.net/blobbercore/datastore"
	"0chain.net/core/chain"
	"0chain.net/core/common"
	"0chain.net/core/encryption"
	"0chain.net/core/node"
	"0chain.net/core/transaction"

	. "0chain.net/core/logging"
	"go.uber.org/zap"
)

type ReadRedeem struct {
	ReadMarker *ReadMarker `json:"read_marker"`
}

func (rm *ReadMarkerEntity) VerifyMarker(ctx context.Context, sa *allocation.Allocation) error {
	if rm == nil || rm.LatestRM == nil {
		return common.NewError("invalid_read_marker", "No read marker was found")
	}
	if rm.LatestRM.AllocationID != sa.ID {
		return common.NewError("read_marker_validation_failed", "Read Marker is not for the same allocation")
	}

	if rm.LatestRM.BlobberID != node.Self.ID {
		return common.NewError("read_marker_validation_failed", "Read Marker is not for the blobber")
	}

	clientPublicKey := ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)
	if len(clientPublicKey) == 0 || clientPublicKey != rm.LatestRM.ClientPublicKey {
		return common.NewError("read_marker_validation_failed", "Could not get the public key of the client")
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || clientID != rm.LatestRM.ClientID {
		return common.NewError("read_marker_validation_failed", "Read Marker clientID does not match request clientID")
	}
	currentTS := common.Now()
	if rm.LatestRM.Timestamp > currentTS {
		Logger.Error("Timestamp is for future in the read marker", zap.Any("rm", rm), zap.Any("now", currentTS))
	}
	currentTS = common.Now()
	if rm.LatestRM.Timestamp > (currentTS + 2) {
		Logger.Error("Timestamp is for future in the read marker", zap.Any("rm", rm), zap.Any("now", currentTS))
		return common.NewError("read_marker_validation_failed", "Timestamp is for future in the read marker")
	}

	hashData := rm.LatestRM.GetHashData()
	signatureHash := encryption.Hash(hashData)
	sigOK, err := encryption.Verify(clientPublicKey, rm.LatestRM.Signature, signatureHash)
	if err != nil {
		return common.NewError("read_marker_validation_failed", "Error during verifying signature. "+err.Error())
	}
	if !sigOK {
		return common.NewError("read_marker_validation_failed", "Read marker signature is not valid")
	}
	return nil
}

// PendNumBlocks return zero, if redeem_required is false. If its true, it
// returns difference between latest redeemed read marker and current one
// (till not redeemed).
func (rme *ReadMarkerEntity) PendNumBlocks() (pendNumBlocks int64, err error) {

	if !rme.RedeemRequired {
		return // (0, nil), everything is already redeemed
	}

	if rme.LatestRM == nil {
		return 0, common.NewErrorf("rme_pend_num_blocks",
			"missing latest read marker (nil)")
	}

	if len(rme.LatestRedeemedRMBlob) == 0 {
		return rme.LatestRM.ReadCounter, nil // the number of blocks read
	}

	// then decode previous read marker
	var prev = new(ReadMarker)
	err = json.Unmarshal(rme.LatestRedeemedRMBlob, prev)
	if err != nil {
		return 0, common.NewErrorf("rme_pend_num_blocks",
			"decoding previous read marker: %v", err)
	}

	pendNumBlocks = rme.LatestRM.ReadCounter - prev.ReadCounter
	return

}

// getNumBlocks to redeem (difference between the previous RM and the
// current one)
func (rme *ReadMarkerEntity) getNumBlocks() (numBlocks int64, err error) {

	if rme.LatestRM == nil {
		return 0, common.NewErrorf("rme_get_num_blocks",
			"missing latest read marker (nil)")
	}

	if len(rme.LatestRedeemedRMBlob) == 0 {
		return rme.LatestRM.ReadCounter, nil // the number of blocks read
	}

	// then decode previous read marker
	var prev = new(ReadMarker)
	err = json.Unmarshal(rme.LatestRedeemedRMBlob, prev)
	if err != nil {
		return 0, common.NewErrorf("rme_get_num_blocks",
			"decoding previous read marker: %v", err)
	}

	numBlocks = rme.LatestRM.ReadCounter - prev.ReadCounter
	return
}

// preRedeem the marker check read pools; is there enough tokens to send
// a redeeming transaction regarding cache, pending reads (regardless since
// pending reads is what we are going to redeem) and requesting 0chain to
// refresh read pools
func (rme *ReadMarkerEntity) preRedeem(ctx context.Context,
	alloc *allocation.Allocation, numBlocks int64) (
	rps []*allocation.ReadPool, err error) {

	// check out read pool tokens if read_price > 0
	var (
		db        = datastore.GetStore().GetTransaction(ctx)
		blobberID = rme.LatestRM.BlobberID //
		clientID  = rme.LatestRM.ClientID  //
		until     = common.Now()           // all pools until now

		want = alloc.WantRead(blobberID, numBlocks)
		have int64
	)

	if want == 0 {
		return // skip if read price is zero
	}

	// create fake pending instance
	rps, err = allocation.ReadPools(db, clientID, alloc.ID, blobberID, until)
	if err != nil {
		return nil, common.NewErrorf("rme_pre_redeem",
			"can't get read pools from DB: %v", err)
	}

	// regardless pending reads
	for _, rp := range rps {
		have += rp.Balance // expired pools was excluded by DB query
	}

	if have < want {
		rps, err = allocation.RequestReadPools(clientID,
			alloc.ID)
		if err != nil {
			return nil, common.NewErrorf("rme_pre_redeem",
				"can't request read pools from sharders: %v", err)
		}
		// cache in DB for next requests
		err = allocation.SetReadPools(db, clientID,
			alloc.ID, blobberID, rps)
		if err != nil {
			return nil, common.NewErrorf("rme_pre_redeem",
				"can't save the requested read pools: %v", err)
		}
		// update the 'have' given from sharders
		for _, rp := range rps {
			if rp.ExpireAt < until {
				continue // excluding all expired pools
			}
			have += rp.Balance
		}
	}

	if have < want {
		// so, not enough tokens, let's freeze the read marker
		err = db.Model(rme).Update("suspend", rme.LatestRM.ReadCounter).Error
		if err != nil {
			return nil, common.NewErrorf("rme_pre_redeem",
				"saving suspended read marker: %v", err)
		}

		return nil, common.NewErrorf("rme_pre_redeem", "not enough tokens "+
			"client -> allocation -> blobber (%s->%s->%s), have: %d, want: %d",
			rme.LatestRM.ClientID, rme.LatestRM.AllocationID,
			rme.LatestRM.BlobberID, have, want)
	}

	return
}

// RedeemReadMarker redeems the read marker.
func (rme *ReadMarkerEntity) RedeemReadMarker(ctx context.Context) (
	err error) {

	if rme.LatestRM.Suspend == rme.LatestRM.ReadCounter {
		// suspended read marker, no tokens in related read pools
		// don't request 0chain to refresh the read pools; let user
		// download more (he is unable to download for now) and the
		// downloading forces the read pools cache refreshing
		return common.NewError("redeem_read_marker",
			"read marker redeeming suspended until next successful download")
	}

	var alloc *allocation.Allocation
	alloc, err = allocation.GetAllocationByID(ctx,
		rme.LatestRM.AllocationID)
	if err != nil {
		return common.NewErrorf("redeem_read_marker",
			"can't get allocation from DB: %v", err)
	}

	// load corresponding terms
	if err = alloc.LoadTerms(ctx); err != nil {
		return common.NewErrorf("redeem_read_marker",
			"can't load allocation terms from DB: %v", err)
	}

	var numBlocks int64
	if numBlocks, err = rme.getNumBlocks(); err != nil {
		return common.NewErrorf("redeem_read_marker",
			"can't get number of blocks read to redeem: %v", err)
	}

	var rps []*allocation.ReadPool
	if rps, err = rme.preRedeem(ctx, alloc, numBlocks); err != nil {
		return common.NewErrorf("redeem_read_marker",
			"pre-redeeming error: %v", err)
	}

	// ok, now we can redeem the marker and then update pools in cache

	var tx *transaction.Transaction
	if tx, err = transaction.NewTransactionEntity(); err != nil {
		return common.NewErrorf("redeem_read_marker",
			"creating transaction: %v", err)
	}

	var (
		rm = rme.LatestRM
		sn = &ReadRedeem{}
	)
	sn.ReadMarker = rm

	var snBytes []byte
	if snBytes, err = json.Marshal(sn); err != nil {
		Logger.Error("Error encoding SC input", zap.Error(err),
			zap.Any("scdata", sn))
		return common.NewErrorf("redeem_read_marker",
			"encoding SC data: %v", err)
	}

	err = tx.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.READ_REDEEM, string(snBytes), 0)
	if err != nil {
		Logger.Info("Failed submitting read redeem", zap.Error(err))
		return common.NewErrorf("redeem_read_marker",
			"sending transaction: %v", err)
	}

	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)

	tx, err = transaction.VerifyTransaction(tx.Hash, chain.GetServerChain())
	if err != nil {
		Logger.Error("Error verifying the read redeem transaction",
			zap.Error(err), zap.String("txn", tx.Hash))
		return common.NewErrorf("redeem_read_marker",
			"verifying transaction: %v", err)
	}

	err = rme.UpdateStatus(ctx, rps, tx.TransactionOutput, tx.Hash)
	if err != nil {
		return common.NewErrorf("redeem_read_marker",
			"updating read marker status: %v", err)
	}

	return // nil, ok
}
