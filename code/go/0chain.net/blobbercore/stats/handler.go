package stats

import (
	"context"
	"html/template"
	"net/http"

	"0chain.net/blobbercore/constants"
	"0chain.net/blobbercore/datastore"
	"0chain.net/core/common"
	. "0chain.net/core/logging"

	"go.uber.org/zap"
)

var funcMap = template.FuncMap{}

const tpl = `<!DOCTYPE html>
<html>
  <head>
    <title>Blobber Diagnostics</title>
  </head>
  <body>
    <h1>
      Blobber Stats
    </h1>
    <table border="1">
      <tr>
        <td>ID</td>
        <td>{{ .ClientID }}</td>
      </tr>
      <tr>
        <td>PublicKey</td>
        <td>{{ .PublicKey }}</td>
      </tr>
      <tr>
        <td>Allocations</td>
        <td>{{ .NumAllocation }}</td>
      </tr>
      <tr>
        <td>Used Size (bytes)</td>
        <td>{{ .UsedSize }}</td>
      </tr>
      <tr>
        <td>Actual Disk Usage (bytes)</td>
        <td>{{ .DiskSizeUsed }}</td>
      </tr>
      <tr>
        <td>Num of files</td>
        <td>{{ .NumWrites }}</td>
      </tr>
	  <tr>
        <td>Blocks Written</td>
        <td>{{ .BlockWrites }}</td>
      </tr>
	  <tr>
        <td>Blocks Read</td>
        <td>{{ .NumReads }}</td>
      </tr>
      <tr>
        <td>Total Challenges</td>
        <td>{{ .TotalChallenges }}</td>
      </tr>
      <tr>
        <td>Open Challenges</td>
        <td>{{ .OpenChallenges }}</td>
      </tr>
      <tr>
        <td>Passed Challenges</td>
        <td>{{ .SuccessChallenges }}</td>
      </tr>
      <tr>
        <td>Failed Challenges</td>
        <td>{{ .FailedChallenges }}</td>
      </tr>
      <tr>
        <td>Redeemed Challenges</td>
        <td>{{ .RedeemedChallenges }}</td>
      </tr>
      <tr>
        <td>Redeemed Challenges</td>
        <td>{{ .RedeemedChallenges }}</td>
      </tr>
      <tr>
        <table>
          <tr><th colspan="2">Configurations</th></tr>
          <tr><td>Capacity</td><td>{{ .Capacity }}</td></tr>
          <tr><td>Read price</td><td>{{ .ReadPrice }}</td></tr>
          <tr><td>Write price</td><td>{{ .WritePrice }}</td></tr>
          <tr><td>Min lock demand</td><td>{{ .MinLockDemand }}</td></tr>
          <tr><td>Max offer duration</td><td>{{ .MaxOfferDuration }}</td></tr>
          <tr><td>Challenge completion_time</td><td>{{ .ChallengeCompletionTime }}</td></tr>
          <tr><td>Read lock timeout</td><td>{{ .ReadLockTimeout }}</td></tr>
          <tr><td>Write lock timeout</td><td>{{ .WriteLockTimeout }}</td></tr>
        </table>
      </tr>
    </table>

    <h1>
      Allocation Stats
	</h1>
	<p>Note: You might not see stats for all allocations. Allocations that have no data will not be collecting stats</p>
    <table border="1">
      <tr>
        <td>ID</td>
        <td>Used Size (bytes)</td>
        <td>Actual Disk Usage (bytes)</td>
        <td>Temp Folder Size (bytes)</td>
        <td>Num of files</td>
        <td>Blocks Written</td>
        <td>Blocks Read</td>
        <td>Total Challenges</td>
        <td>Open Challenges</td>
        <td>Passed Challenges</td>
        <td>Failed Challenges</td>
        <td>Redeemed Challenges</td>
        <td>Expiration</td>
      </tr>
      {{range .AllocationStats}}

      <tr>
        <td>{{ .AllocationID }}</td>
        <td>{{ .UsedSize }}</td>
        <td>{{ .DiskSizeUsed }}</td>
        <td>{{ .TempFolderSize }}</td>
        <td>{{ .NumWrites }}</td>
        <td>{{ .BlockWrites }}</td>
        <td>{{ .NumReads }}</td>
        <td>{{ .TotalChallenges }}</td>
        <td>{{ .OpenChallenges }}</td>
        <td>{{ .SuccessChallenges }}</td>
        <td>{{ .FailedChallenges }}</td>
        <td>{{ .RedeemedChallenges }}</td>
        <td>{{ .Expiration }}</td>
      </tr>
      {{end}}
    </table>
  </body>
</html>
`

func StatsHandler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.New("diagnostics").Funcs(funcMap).Parse(tpl))
	ctx := datastore.GetStore().CreateTransaction(r.Context())
	db := datastore.GetStore().GetTransaction(ctx)
	defer db.Rollback()
	bs := LoadBlobberStats(ctx)
	err := t.Execute(w, bs)
	if err != nil {
		Logger.Error("Error in executing the template", zap.Error(err))
	}
}

func StatsJSONHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	ctx = datastore.GetStore().CreateTransaction(ctx)
	db := datastore.GetStore().GetTransaction(ctx)
	defer db.Rollback()
	bs := LoadBlobberStats(ctx)
	return bs, nil
}

func GetStatsHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	q := r.URL.Query()
	ctx = context.WithValue(ctx, constants.ALLOCATION_CONTEXT_KEY, q.Get("allocation_id"))
	ctx = datastore.GetStore().CreateTransaction(ctx)
	db := datastore.GetStore().GetTransaction(ctx)
	defer db.Rollback()
	allocationID := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	bs := &BlobberStats{}
	if len(allocationID) != 0 {
		// TODO: Get only the allocation info from DB
		bs.loadDetailedStats(ctx)
		for _, allocStat := range bs.AllocationStats {
			if allocStat.AllocationID == allocationID {
				return allocStat, nil
			}
		}
		return nil, common.NewError("allocation_stats_not_found", "Stats for allocation not found")
	}
	allocations := q.Get("allocations")
	if len(allocations) != 0 {
		return loadAllocationList(ctx)
	}
	bs.loadBasicStats(ctx)
	return bs, nil
}
