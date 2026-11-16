package results

import (
	"net/http"
	"strconv"

	"github.com/librespeed/speedtest-go/database"
	"github.com/librespeed/speedtest-go/database/mongodb"
	log "github.com/sirupsen/logrus"
)

// BarometerHandler situe un résultat dans la base des mesures existantes
// (IMP-09 — baromètre de comparaison, façon nPerf ; ancien libellé V4 : IMP-11).
//
// GET /api/dashboard/barometer?download=12.4&upload=2.1&ping=45
//   [&operator=Orange][&netType=wifi|mobile][&tech=4G]
//   [&streaming=88][&browsing=95][&lat=12.37&lng=-1.52][&deviceId=…]
//
// Réponse :
//   - download/uploadPercentile : pourcentage de mesures dont le débit est
//     strictement inférieur au résultat testé ;
//   - pingPercentile : pourcentage de mesures dont la latence est pire
//     (supérieure) au résultat testé ;
//   - sampleSize : nombre de mesures actives abouties prises en compte
//     (déjà segmentées par netType) ;
//   - medianDownload/medianUpload/medianPing : médianes de la base comparée,
//     pour l'affichage sur les barres ;
//   - downloadRank/uploadRank/pingRank : rang du résultat (1 = le plus
//     rapide ; nombre de mesures battues + 1) ;
//   - streaming/browsingPercentile : mêmes lectures pour les scores des
//     tests de streaming et de navigation (omis si le testeur ne les a pas
//     faits) ;
//   - operators : classement des opérateurs par débit moyen descendant ;
//   - nationalAvgDownload : débit descendant moyen de toutes les mesures ;
//   - operator* (si `operator` est fourni) : mêmes centiles parmi les seuls
//     clients de cet opérateur, et leur débit moyen (au moins 5 mesures) ;
//   - operatorTech* : sous-groupe opérateur × techno (au moins 10 mesures) ;
//   - zone* : sous-groupe géographique, ~10 km autour du test (au moins
//     10 mesures) ;
//   - recentDownloads : 5 derniers tests du même appareil (deviceId), du
//     plus récent au plus ancien.
func BarometerHandler(w http.ResponseWriter, r *http.Request) {
	if !mongoRequired(w) {
		return
	}
	var p mongodb.BarometerParams
	p.Download, _ = strconv.ParseFloat(r.URL.Query().Get("download"), 64)
	p.Upload, _ = strconv.ParseFloat(r.URL.Query().Get("upload"), 64)
	p.Ping, _ = strconv.ParseFloat(r.URL.Query().Get("ping"), 64)
	p.Streaming, _ = strconv.ParseFloat(r.URL.Query().Get("streaming"), 64)
	p.Browsing, _ = strconv.ParseFloat(r.URL.Query().Get("browsing"), 64)
	p.Lat, _ = strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	p.Lng, _ = strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	p.Operator = r.URL.Query().Get("operator")
	p.NetworkType = r.URL.Query().Get("netType")
	p.Tech = r.URL.Query().Get("tech")
	p.DeviceID = r.URL.Query().Get("deviceId")

	res, err := database.MongoDB.FetchBarometer(p)
	if err != nil {
		log.Errorf("BarometerHandler error: %s", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, res)
}
