package storage

import (
	"fmt"
	"net/http"
)

type Stats struct {
	TotalChallenges      int
	SuccessfulChallenges int
	FailedChallenges     int
	// Add more fields as needed
}

var appStats Stats

func init() {
	appStats = Stats{}
}

func updateStats(success bool) {
	// Process the request

	appStats.TotalChallenges++

	if success {
		appStats.SuccessfulChallenges++
	} else {
		appStats.FailedChallenges++
	}
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	// You might want to use a template engine to format the HTML
	// Here, I'm using a simple text format for demonstration
	statsText := fmt.Sprintf("Total Requests: %d\nSuccessful Requests: %d\nFailed Requests: %d\n",
		appStats.TotalChallenges, appStats.SuccessfulChallenges, appStats.FailedChallenges)

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(statsText))
}
