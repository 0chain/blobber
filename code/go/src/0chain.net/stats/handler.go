package stats

import (
	"html/template"
	"net/http"

	. "0chain.net/logging"

	"go.uber.org/zap"
)

func StatsHandler(w http.ResponseWriter, r *http.Request) {
	// Files are provided as a slice of strings.
	paths := []string{
		"templates/diagnostics.tmpl",
	}
	t := template.Must(template.New("diagnostics.tmpl").ParseFiles(paths...))
	ctx := GetStatsStore().WithReadOnlyConnection(r.Context())
	defer GetStatsStore().Discard(ctx)
	bs := LoadBlobberStats(ctx)
	err := t.Execute(w, bs)
	if err != nil {
		Logger.Error("Error in executing the template", zap.Error(err))
	}
}
