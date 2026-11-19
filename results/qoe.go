package results

import (
	"net/http"
	"strings"

	"github.com/librespeed/speedtest-go/database"
	"github.com/librespeed/speedtest-go/database/mongodb"
	log "github.com/sirupsen/logrus"
)

// qoeFilterFromRequest lit les filtres communs à l'onglet « Qualité perçue »
// et à la couche carte : réseau, opérateur, techno, période.
func qoeFilterFromRequest(r *http.Request) mongodb.QoeFilter {
	q := r.URL.Query()
	return mongodb.QoeFilter{
		Reseau:   strings.ToLower(strings.TrimSpace(q.Get("reseau"))),
		Operator: strings.TrimSpace(q.Get("operator")),
		Tech:     strings.TrimSpace(q.Get("tech")),
		Period:   strings.TrimSpace(q.Get("period")),
		From:     strings.TrimSpace(q.Get("from")),
		To:       strings.TrimSpace(q.Get("to")),
	}
}

// DashboardQoe returns aggregated perceived-quality stats.
// GET /api/dashboard/qoe?reseau=mobile&operator=&tech=&period=month&from=&to=
func DashboardQoe(w http.ResponseWriter, r *http.Request) {
	if !mongoRequired(w) {
		return
	}
	stats, err := database.MongoDB.FetchQoeStats(qoeFilterFromRequest(r))
	if err != nil {
		log.Errorf("DashboardQoe error: %s", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, stats)
}
