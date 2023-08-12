package storage

import (
	"fmt"
	"net/http"
)

type Stats struct {
	TotalChallenges      int
	SuccessfulChallenges int
	FailedChallenges     int
}

var appStats Stats

func init() {
	appStats = Stats{}
}

func updateStats(success bool) {
	appStats.TotalChallenges++

	if success {
		appStats.SuccessfulChallenges++
	} else {
		appStats.FailedChallenges++
	}
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
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
				<td>` + fmt.Sprintf("%d", appStats.TotalChallenges) + `</td>
			</tr>
			<tr>
				<td>Successful Challenges</td>
				<td>` + fmt.Sprintf("%d", appStats.SuccessfulChallenges) + `</td>
			</tr>
			<tr>
				<td>Failed Challenges</td>
				<td>` + fmt.Sprintf("%d", appStats.FailedChallenges) + `</td>
			</tr>
		</table>
	</body>
	</html>
	`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(statsHTML))
}
