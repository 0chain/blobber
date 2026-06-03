# StorageV2 wmpt dangling-node bug — `pebble: not found` on reference path

**Status:** root cause confirmed, fix written + proven by regression test in `0chain/common`.
**Affected:** mainnet blobbers running **StorageV2** (weighted merkle patricia trie persisted in Pebble). **Not** eblobbers (see Scope).
**Date:** 2026-06-03

## Symptom

After a sequence of write/delete commits on a StorageV2 allocation, `GET /v2/file/referencepath/...` starts returning:

```
400 {"code":"invalid_parameters","error":"invalid_parameters: Invalid path. pebble: not found"}
```

Every subsequent write (upload/delete/move) on that allocation fails with `commit_failed: Failed to get reference path ... pebble: not found`. It does **not** self-heal on restart (the bad root is persisted in `allocations.allocation_root`), and it recurs after a `recover_trie` because the *next* batch of deletes re-introduces it. Most often triggered by deleting directory trees (e.g. HLS/streaming `.mp4` dirs), where churn repeatedly shifts trie depth across the collapse boundary.

## Root cause

The bug is in the merkle-trie library `github.com/0chain/common`, `core/util/wmpt/trie.go`, function `commit()`.

The persist lifecycle (blobber `CommitWriteV2` → `handler_common.go`) is:

```
trie.Update(...)                 // mutate in-memory trie
batcher = trie.Commit(COLLAPSE_DEPTH)   // walk dirty nodes, Save() to batch, collapse below depth to hashNodes
batcher.Commit(true)             // write new nodes to Pebble
tx.Commit()                      // persist new root in postgres
trie.DeleteNodes()               // prune OLD nodes from Pebble (deferred one commit)
```

`Commit()` tracks two sets via channels while walking dirty nodes:

- **`createdChan`** — nodes written this commit. `collectDeleteAndCreated` uses it to (a) clean up writes on rollback, and **critically (b) remove a hash from the pending-delete set** (`delete(t.deleted, k)`), so a node that is deleted-then-recreated is not pruned.
- **`deleteChan`** — old node hashes (`prevHash`), deferred one commit, then physically deleted by `DeleteNodes()`.

A node is protected from deletion **only if its hash arrives on `createdChan`**. Every node path sends it — root (`trie.go:411`), non-collapsed routing node (`493`), shortNode (`518`), valueNode (`529`) — **except** a `routingNode` sitting exactly at `collapseLevel`:

```go
// core/util/wmpt/trie.go  (routingNode case in commit())
err = n.Save(batcher)            // node IS written to Pebble
...
if level == collapseLevel {
    n.Children = [16]Node{}
    return &hashNode{hash: n.Hash(), weight: n.Weight()}, nil   // <-- returns WITHOUT createdChan <- n.Hash()
}
createdChan <- n.Hash()
```

Because the collapsed node's hash never reaches `createdChan`, it is never removed from the pending-delete set. The trie is content-addressed, so bulk deletes routinely cause a node whose hash was queued for deletion in an earlier commit (`tempDeleted → t.deleted`) to **re-appear at the collapse boundary** in a later commit (delete churn changes tree depth). On the next `DeleteNodes()` that hash — **still referenced by the new root via a `hashNode`** — is pruned from Pebble. The reference-path walk later does `db.Get(hash)` → `pebble: not found`.

Introduced in `edb7935a` (original wmpt, 2024-09-04); became corrupting once delete-pruning (`deleteChan <- prevHash`) was added in `d39d2e0d` (2025-03-28). `COLLAPSE_DEPTH = 3` (blobber `filestore/store.go`), so the boundary is hit constantly.

## Fix

Register the collapsed node as created, exactly like every other written node (`core/util/wmpt/trie.go`):

```go
if level == collapseLevel {
    createdChan <- n.Hash()   // protect collapsed node from the deferred DeleteNodes() prune; clean it on rollback
    n.Children = [16]Node{}
    return &hashNode{hash: n.Hash(), weight: n.Weight()}, nil
}
```

One line. The node is already `Save()`d; this only marks it "created this commit," matching its siblings. No behavioral downside; it also closes a (previously silent) rollback leak of collapse-boundary nodes.

## Regression test

`TestCommitCollapsePruneDangling` (in `core/util/wmpt/trie_test.go`): builds a multi-level trie, then runs delete/re-add churn committing + `DeleteNodes()` each round, reloading from the persisted root and calling `GetPath` for all surviving keys. **Proven: FAILS without the fix (`pebble: not found`), PASSES with it.** Existing `TestTrieCommit`/`TestRollbackTrie` still pass.

## Scope

- **Mainnet blobbers (StorageV2, `common v1.20.1`):** affected. Need the fixed `common` → rebuilt blobber image → deploy.
- **gosdk:** imports `common/wmpt` to compute roots client-side (`ProcessChangeV2` in upload/delete/rename workers) but does not run the `Commit→batcher→DeleteNodes` persist/prune path, so it is not where the bug bites. It picks up the fixed library on its next build; no urgency.
- **eblobbers (`0chain/eblobber`):** **not affected.** They pin `common v0.0.6` (Jan 2023, predates wmpt) and contain no wmpt / Pebble / StorageV2 code — legacy storage model, no merkle trie.

## Deployment

1. Land the fix in `0chain/common` (merge into the ref the blobber builds against) and bump the blobber's `go.mod`.
2. Build a new blobber image; deploy to all mainnet blobbers.
3. On that same boot, run `--recover_trie` with `recover_allocations: [<affected-alloc-ids>]` to rebuild any already-corrupted tries from `reference_objects` (re-persisting every node) — this clears existing `pebble: not found`. Until the fixed binary is deployed, `recover_trie` is only a temporary fix because the next delete batch re-corrupts the trie.

## Operational recovery (band-aid, pre-deploy)

`Allocation.recoverTrie()` (`-recover_trie` flag + `recover_allocations` config) rebuilds an allocation's trie from its FILE refs and re-persists all nodes, fixing the dangling reference for that allocation. It does not prevent recurrence — only the code fix does.
