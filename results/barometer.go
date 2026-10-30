package results

import (
	"net/http"
	"strconv"

	"github.com/librespeed/speedtest-go/database"
	log "github.com/sirupsen/logrus"
)

// BarometerHandler situe un résultat dans la base des mesures existantes
// (IMP-09 — baromètre de comparaison, façon nPerf ; ancien libellé V4 : IMP-11).
//
// GET /api/dashboard/barometer?download=12.4&upload=2.1&ping=45[&operator=Orange]
//
// Réponse :
//   - download/uploadPercentile : pourcentage de mesures dont le débit est
//     strictement inférieur au résultat testé ;
//   - pingPercentile : pourcentage de mesures dont la latence est pire
//     (supérieure) au résultat testé ;
//   - sampleSize : nombre de mesures actives abouties prises en compte ;
//   - operators : classement des opérateurs par débit moyen descendant ;
//   - nationalAvgDownload : débit descendant moyen de toutes les mesures ;
//   - operator* (si `operator` est fourni) : mêmes centiles parmi les seuls
//     clients de cet opérateur, et leur débit moyen (au moins 5 mesures).
func BarometerHandler(w http.ResponseWriter, r *http.Request) {
	if !mongoRequired(w) {
		return
	}
	download, _ := strconv.ParseFloat(r.URL.Query().Get("download"), 64)
	upload, _ := strconv.ParseFloat(r.URL.Query().Get("upload"), 64)
	ping, _ := strconv.ParseFloat(r.URL.Query().Get("ping"), 64)

	operator := r.URL.Query().Get("operator")

	res, err := database.MongoDB.FetchBarometer(download, upload, ping, operator)
	if err != nil {
		log.Errorf("BarometerHandler error: %s", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, res)
}
