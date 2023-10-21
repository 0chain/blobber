package stats

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/constants"
	"go.uber.org/zap"
)

func byteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func byteCountIEC2(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

const (
	Ki            = 1024    // kilobyte
	readBlockSize = 64 * Ki // read block size is 64 KiB
)

var funcMap = template.FuncMap{
	"read_size": func(readCount int64) string {
		return byteCountIEC(readCount * readBlockSize)
	},
	"write_size":                      byteCountIEC,
	"byte_count_in_string":            byteCountIEC,
	"byte_count_in_string_for_uint64": byteCountIEC2,
	"time_in_string": func(timeValue time.Time) string {
		if timeValue.IsZero() {
			return "-"
		}
		return timeValue.Format(DateTimeFormat)
	},
}

const tpl = `
<table style='border-collapse: collapse;'>
    <tr class='header'>
        <td>Summary</td>
        <td>Configurations</td>
        <td>Markers</td>
        <td>Infra Stats</td>
        <td>Database</td>
    </tr>
    <tr>
        <td>
            <table class='menu' style='border-collapse: collapse;'>
                <tr>
                    <td>Allocations</td>
                    <td>{{ .NumAllocation }}</td>
                </tr>
                <tr>
                    <td>Allocated size</td>
                    <td>{{ byte_count_in_string .AllocatedSize }}</td>
                </tr>
                <tr>
                    <td>Used Size</td>
                    <td>{{ byte_count_in_string .UsedSize }}</td>
                </tr>
                <tr>
                    <td>Actual Disk Usage</td>
                    <td>{{ byte_count_in_string_for_uint64 .DiskSizeUsed }}</td>
                </tr>
                <tr>
                    <td>Files Size</td>
                    <td>{{ byte_count_in_string .FilesSize }}</td>
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
            </table>
        </td>
        <td valign='top'>
            <table class='menu' style='border-collapse: collapse;'>
                <tr><td>Capacity</td><td>{{ byte_count_in_string .Capacity }}</td></tr>
                <tr><td>Read price</td><td>{{ .ReadPrice }}</td></tr>
                <tr><td>Write price</td><td>{{ .WritePrice }}</td></tr>
            </table>
        </td>
        <td valign='top'>
            <table class='menu' style='border-collapse: collapse;'>
                <tr><td colspan='3'>Read Markers</td></tr>
                <tr>
                    <td>Pending</td>
                    <td>{{ .ReadMarkers.Pending }} <i>(64 KB blocks)</i></td>
                    <td>{{ read_size .ReadMarkers.Pending }}</td>
                </tr>
                <tr>
                    <td>Redeemed</td>
                    <td>{{ .ReadMarkers.Redeemed }} <i>(64 KB blocks)</i></td>
                    <td>{{ read_size .ReadMarkers.Redeemed }}</td>
                </tr>

                <tr><td colspan='3'></td></tr>
                <tr><td colspan='3'>Write Markers</td></tr>
                <tr>
                    <td>Accepted</td>
                    <td>{{ .WriteMarkers.Accepted.Count }} markers</td>
                    <td>{{ write_size .WriteMarkers.Accepted.Size }}</td>
                </tr>
                <tr>
                    <td>Committed</td>
                    <td>{{ .WriteMarkers.Committed.Count }} markers</td>
                    <td>{{ write_size .WriteMarkers.Committed.Size }}</td>
                </tr>
                <tr>
                    <td>Failed</td>
                    <td>{{ .WriteMarkers.Failed.Count }} markers</td>
                    <td>{{ write_size .WriteMarkers.Failed.Size }}</td>
                </tr>
            </table>
        </td>
        <td valign='top'>
            <table class='menu' style='border-collapse: collapse;'>
                <tr>
                    <td>CPU</td>
                    <td>{{ .InfraStats.CPUs }}</td>
                </tr>
                <tr>
                    <td>Go Routines</td>
                    <td>{{ .InfraStats.NumberOfGoroutines }}</td>
                </tr>
                <tr>
                    <td>Heap Sys</td>
                    <td>{{ byte_count_in_string .InfraStats.HeapSys }}</td>
                </tr>
                <tr>
                    <td>Heap Alloc</td>
                    <td>{{ byte_count_in_string .InfraStats.HeapAlloc }}</td>
                </tr>
                <tr>
                    <td>Is Active On Chain</td>
                    <td>{{ .InfraStats.ActiveOnChain }}</td>
                </tr>
            </table>
        </td>
        <td valign='top'>
            <table class='menu' style='border-collapse: collapse;'>
                <tr>
                    <td>Open Connections</td>
                    <td>{{ .DBStats.OpenConnections }}</td>
                </tr>
                <tr>
                    <td>Connections In Use</td>
                    <td>{{ .DBStats.InUse }}</td>
                </tr>
                <tr>
                    <td>Idle Connections</td>
                    <td>{{ .DBStats.Idle }}</td>
                </tr>
                <tr>
                    <td>Connection Wait Count</td>
                    <td>{{ .DBStats.WaitCount }}</td>
                </tr>
                <tr>
                    <td>Connection Wait Duration</td>
                    <td>{{ .DBStats.WaitDuration }}</td>
                </tr>
                <tr>
                    <td>Max Open Connections</td>
                    <td>{{ .DBStats.MaxOpenConnections }}</td>
                </tr>
                <tr>
                    <td>Connection Max Idle Closed</td>
                    <td>{{ .DBStats.MaxIdleClosed }}</td>
                </tr>
                <tr>
                    <td>Connection Max Idle Time Closed</td>
                    <td>{{ .DBStats.MaxIdleTimeClosed }}</td>
                </tr>
                <tr>
                    <td>Connection Max Lifetime Closed</td>
                    <td>{{ .DBStats.MaxLifetimeClosed }}</td>
                </tr>
                <tr>
                    <td>Database Status</td>
                    <td>{{ .DBStats.Status }}</td>
                </tr>
            </table>
        </td>
    </tr>
</table>

<h1>
    Allocation Stats
</h1>

<p>Note: You might not see stats for all allocations. Allocations that have no data will not be collecting stats</p>

<div style='position: absolute'>
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
        <td rowspan=2>{{ .AllocationID }}</td>
        <td>{{ byte_count_in_string .UsedSize }}</td>
        <td>{{ byte_count_in_string_for_uint64 .DiskSizeUsed }}</td>
        <td>{{ byte_count_in_string_for_uint64 .TempFolderSize }}</td>
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
    <tr>
        <td colspan=6>
            <table class='menu' style='border-collapse: collapse;'>
                {{ if .ReadMarkers }}
                <tr><th colspan="2">Read markers</th></tr>
                <tr>
                    <td>Pending</td>
                    <td>{{ .ReadMarkers.Pending }} <i>(64 KB blocks)</i></td>
                    <td>{{ read_size .ReadMarkers.Pending }}</td>
                </tr>
                <tr>
                    <td>Redeemed</td>
                    <td>{{ .ReadMarkers.Redeemed }} <i>(64 KB blocks)</i></td>
                    <td>{{ read_size .ReadMarkers.Redeemed }}</td>
                </tr>
                {{ else }}
                <tr><th>No read markers yet.</th></tr>
                {{ end }}
            </table>
        </td>
        <td colspan=6>
            <table class='menu' style='border-collapse: collapse;'>
                {{ if .WriteMarkers }}
                <tr><th colspan="3">Write markers</th></tr>
                <tr>
                    <td>Accepted</td>
                    <td>{{ .WriteMarkers.Accepted.Count }} markers</td>
                    <td>{{ write_size .WriteMarkers.Accepted.Size }}</td>
                </tr>
                <tr>
                    <td>Committed</td>
                    <td>{{ .WriteMarkers.Committed.Count }} markers</td>
                    <td>{{ write_size .WriteMarkers.Committed.Size }}</td>
                </tr>
                <tr>
                    <td>Failed</td>
                    <td>{{ .WriteMarkers.Failed.Count }} markers</td>
                    <td>{{ write_size .WriteMarkers.Failed.Size }}</td>
                </tr>
                {{ else }}
                <tr><th>No write markers yet</th></tr>
                {{ end }}
            </table>
        </td>
    </tr>
    {{end}}
</table>

<div style='margin-right: 25px'>
	<ul class="pagination">
	{{if .AllocationListPagination}}
		{{if .AllocationListPagination.HasPrev}}
		<li class="page-item">
			<a class="page-link" href="?alp={{.AllocationListPagination.PrevPage}}"> << Previous </a>
		</li>
		{{end}}
		{{if gt .AllocationListPagination.TotalPages 1}}
		<li class="page-item">
			<a class="page-link" href="?fcp={{.AllocationListPagination.CurrentPage}}"> {{.AllocationListPagination.CurrentPage}}/{{.AllocationListPagination.TotalPages}} </a>
		</li>
		{{end}}
		{{if .AllocationListPagination.HasNext}}
		<li class="page-item">
			<a class="page-link" href="?alp={{.AllocationListPagination.NextPage}}"> Next >> </a>
		</li>
		{{end}}
	{{end}}
	</ul>
</div>

<br>

<h1>
    Failed Challenges
</h1>

<table style='border-collapse: collapse;'>
	<tr class='header'>
		<td>ID</td>
		<td>Other IDs</td>
		<td>Info</td>
		<td>Failed At</td>
		<td>Message</td>
	</tr>
	{{range .FailedChallengeList}}
	<tr>
		<td>{{ .ChallengeID }}</td>
		<td align='center'>
			<table class='menu' style='border-collapse: collapse;margin-top:10px;margin-bottom:10px;margin-left:5px;margin-right:5px'>
				<tr>
					<td>Allocation ID</td>
					<td>{{ .AllocationID }}</td>
				</tr>
				<tr>
					<td>Allocation Root</td>
					<td>{{ .AllocationRoot }}</td>
				</tr>
				<tr>
					<td>Previous Challenge ID</td>
					<td>{{ .PrevChallengeID }}</td>
				</tr>
			</table>
		</td>
		<td align='center'>
			<table class='menu' style='border-collapse: collapse;margin:5px'>
				<tr>
					<td>Block Number</td>
					<td>{{ .BlockNum }}</td>
				</tr>
				<tr>
					<td>Result</td>
					<td>{{ .Result }}</td>
				</tr>
				<tr>
					<td>Status</td>
					<td>{{ .Status }}</td>
				</tr>
			</table>
		</td>
		<td>{{ time_in_string .UpdatedAt }}</td>
		<td>{{ .StatusMessage }}</td>
	</tr>
	{{end}}
	<tr>
</table>

<div style='margin-right: 25px'>
	<ul class="pagination">
	{{if .FailedChallengePagination}}
		{{if .FailedChallengePagination.HasPrev}}
		<li class="page-item">
			<a class="page-link" href="?fcp={{.FailedChallengePagination.PrevPage}}"> << Previous </a>
		</li>
		{{end}}
		{{if gt .FailedChallengePagination.TotalPages 1}}
		<li class="page-item">
			<a class="page-link" href="?fcp={{.FailedChallengePagination.CurrentPage}}"> {{.FailedChallengePagination.CurrentPage}}/{{.FailedChallengePagination.TotalPages}} </a>
		</li>
		{{end}}
		{{if .FailedChallengePagination.HasNext}}
		<li class="page-item">
			<a class="page-link" href="?fcp={{.FailedChallengePagination.NextPage}}"> Next >> </a>
		</li>
		{{end}}
	{{end}}
	</ul>
</div>

`

func StatsHandler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.New("diagnostics").Funcs(funcMap).Parse(tpl))
	ctx := setStatsRequestDataInContext(context.TODO(), r)
	ctx = datastore.GetStore().CreateTransaction(ctx)
	var bs *BlobberStats
	_ = datastore.GetStore().WithTransaction(ctx, func(ctx context.Context) error {
		bs = LoadBlobberStats(ctx)
		return common.NewError("rollback", "read only")
	})

	err := t.Execute(w, bs)
	if err != nil {
		Logger.Error("Error in executing the template", zap.Error(err))
	}
}

func setStatsRequestDataInContext(ctx context.Context, r *http.Request) context.Context {
	ctx = context.WithValue(ctx, HealthDataKey, r.Header.Get(HealthDataKey.String()))

	allocationPage := r.URL.Query().Get("alp")
	alPageLimitOffset, err := GetPageLimitOffsetFromRequestData(allocationPage)
	if err != nil {
		Logger.Error("setStatsRequestDataInContext", zap.Error(err))
		return ctx
	}
	alrd := RequestData{
		Page:   alPageLimitOffset.Page,
		Limit:  alPageLimitOffset.Limit,
		Offset: alPageLimitOffset.Offset,
	}
	ctx = context.WithValue(ctx, AllocationListRequestDataKey, alrd)

	failedChallengePage := r.URL.Query().Get("fcp")
	fcPageLimitOffset, err := GetPageLimitOffsetFromRequestData(failedChallengePage)
	if err != nil {
		Logger.Error("setStatsRequestDataInContext", zap.Error(err))
		return ctx
	}
	fcrd := RequestData{
		Page:   fcPageLimitOffset.Page,
		Limit:  fcPageLimitOffset.Limit,
		Offset: fcPageLimitOffset.Offset,
	}
	ctx = context.WithValue(ctx, FailedChallengeRequestDataKey, fcrd)

	return ctx
}

func StatsJSONHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	bs := LoadBlobberStats(ctx)
	return bs, nil
}

func GetStatsHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	q := r.URL.Query()
	ctx = context.WithValue(ctx, constants.ContextKeyAllocation, q.Get("allocation_id"))
	allocationID := ctx.Value(constants.ContextKeyAllocation).(string)
	bs := &BlobberStats{}
	if allocationID != "" {
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
	if allocations != "" {
		return loadAllocationList(ctx)
	}
	bs.loadBasicStats(ctx)
	return bs, nil
}
