package stats

import (
	"html/template"
	"net/http"

	. "0chain.net/logging"

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
        <td>Capacity - from config (bytes)</td>
        <td>{{ .Capacity }}</td>
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
        <td>Write Markers</td>
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
        <td>{{ .RedeemSuccessChallenges }}</td>
      </tr>
      <tr>
        <td>Redeem Errored Challenges</td>
        <td>{{ .RedeemErrorChallenges }}</td>
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
		<td>Write Markers</td>
		<td>Blocks Written</td>
        <td>Blocks Read</td>
        <td>Total Challenges</td>
        <td>Open Challenges</td>
        <td>Passed Challenges</td>
        <td>Failed Challenges</td>
        <td>Redeemed Challenges</td>
        <td>Redeem Errored Challenges</td>
        <td>Given Up Challenges</td>
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
        <td>{{ .RedeemSuccessChallenges }}</td>
        <td>{{ .RedeemErrorChallenges }}</td>
        <td>{{ range $key, $value := .GivenUpChallenges }}
             <p>{{ $key }}</p>
            {{ end }}
        </td>
      </tr>
      {{end}}
    </table>
  </body>
</html>
`

func StatsHandler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.New("diagnostics").Funcs(funcMap).Parse(tpl))
	ctx := GetStatsStore().WithReadOnlyConnection(r.Context())
	defer GetStatsStore().Discard(ctx)
	bs := LoadBlobberStats(ctx)
	err := t.Execute(w, bs)
	if err != nil {
		Logger.Error("Error in executing the template", zap.Error(err))
	}
}
