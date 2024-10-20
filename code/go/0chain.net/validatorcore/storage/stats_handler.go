package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"net/http"
	"sync"
)

var Last5Transactions []interface{}

type Stats struct {
	TotalChallenges      int
	SuccessfulChallenges int
	FailedChallenges     int
}

var (
	appStats   Stats
	statsMutex sync.Mutex
)

func init() {
	appStats = Stats{}

}

func updateStats(success bool) {
	statsMutex.Lock()
	defer statsMutex.Unlock()

	appStats.TotalChallenges++

	if success {
		appStats.SuccessfulChallenges++
	} else {
		appStats.FailedChallenges++
	}
}

func getStats() Stats {
	statsMutex.Lock()
	defer statsMutex.Unlock()
	return appStats
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	result := getStats()

	statsHTML := `
	<!DOCTYPE html>
	<html>
	<head>
		<style>
			table {
				font-family: Arial, sans-serif;
				border-collapse: collapse;
				width: 50%;
				margin: auto;
				margin-top: 50px;
			}

			th, td {
				border: 1px solid #dddddd;
				text-align: left;
				padding: 8px;
			}

			th {
				background-color: #f2f2f2;
			}
		</style>
	</head>
	<body>
		<h1>Challenges Statistics</h1>
		<table>
			<tr>
				<th>Statistic</th>
				<th>Count</th>
			</tr>
			<tr>
				<td>Total Challenges</td>
				<td>` + fmt.Sprintf("%d", result.TotalChallenges) + `</td>
			</tr>
			<tr>
				<td>Successful Challenges</td>
				<td>` + fmt.Sprintf("%d", result.SuccessfulChallenges) + `</td>
			</tr>
			<tr>
				<td>Failed Challenges</td>
				<td>` + fmt.Sprintf("%d", result.FailedChallenges) + `</td>
			</tr>
		</table>
	 <div class="transactions">
            <h2>Last 5 Transactions</h2>
            <ul>
    `
	for _, transaction := range Last5Transactions {
		jsonData, err := json.Marshal(transaction)
		if err != nil {
			statsHTML += "<li>Failed to marshal transaction</li>"
			continue
		}
		statsHTML += "<li>" + string(jsonData) + "</li>"
	}
	statsHTML += `
            </ul>
        </div>
    </body>
    </html>
    `

	w.Header().Set("Content-Type", "text/html")
	_, err := w.Write([]byte(statsHTML))
	if err != nil {
		return
	}
}

func validatorInfoHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return map[string]string{
		"validator_id": node.Self.ID,
	}, nil
}
