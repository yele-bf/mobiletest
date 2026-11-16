package mongodb

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// minOperatorSample : en dessous, la comparaison par opérateur n'est pas
// renvoyée — un centile calculé sur trois mesures ne situe rien.
const minOperatorSample = 5

// minSubgroupSample : en dessous, les sous-groupes opérateur × techno et
// « zone » ne sont pas renvoyés. Ils sont plus fins que le groupe
// opérateur, donc exigeant : dix mesures est le strict minimum pour situer
// un résultat sans afficher des pourcentages sans valeur.
const minSubgroupSample = 10

// zoneRadiusKm : rayon du sous-groupe « zone » — un quartier ou un bassin
// de vie, pas la ville entière.
const zoneRadiusKm = 10

// BarometerParams : le résultat testé et les filtres de comparaison.
//
//	Download/Upload en Mb/s, Ping en ms ; Streaming/Browsing : scores 0-100
//	du test de streaming et de navigation, 0 = test non fait (aucun centile
//	n'est alors renvoyé pour cette métrique).
//
//	NetworkType : segment de la base, "wifi" ou "mobile" — un test Wi-Fi
//	n'est pas comparé à des mesures 4G. Vide = toutes les mesures.
//
//	Tech : techno radio du testeur ("4G"…) ; le sous-groupe opérateur ×
//	techno n'est calculé que si fourni et que si l'échantillon suffit.
//
//	Lat/Lng : centre de la zone (~10 km). 0,0 (ou l'un des deux seul) = pas
//	de comparaison de zone — acceptable au Burkina Faso, où aucune mesure
//	ne tombe au point 0 de la mer.
//
//	DeviceID : identifiant d'appareil (IMP-13) pour la tendance sur les 5
//	derniers tests du même appareil.
type BarometerParams struct {
	Download, Upload, Ping float64
	Streaming, Browsing    float64
	Operator               string
	NetworkType            string
	Tech                   string
	Lat, Lng               float64
	DeviceID               string
}

// hasZone : vrai si le testeur a transmis une position exploitable.
func (p BarometerParams) hasZone() bool {
	return p.Lat != 0 || p.Lng != 0
}

// barometerFrom calcule le baromètre (IMP-09) sur des documents déjà chargés.
// Séparé de FetchBarometer pour être testé sans base MongoDB.
//
// Centiles : part des mesures « battues » par le résultat testé (débit
// strictement inférieur ; latence strictement supérieure ; score de
// streaming ou de navigation strictement inférieur). Si [Operator] est
// fourni, les mêmes centiles sont calculés sur les seules mesures de cet
// opérateur, reconnues par operatorFamily() — éventuellement restreintes à
// la même techno ([Tech], au moins minSubgroupSample mesures). Une zone de
// zoneRadiusKm km autour du test est comparée de la même façon.
//
// Les docs doivent être triés du plus récent au plus ancien (c'est l'ordre
// renvoyé par FetchBarometer) : la tendance lit les 5 derniers tests du
// même appareil dans cet ordre.
func barometerFrom(docs []Document, p BarometerParams) *BarometerResult {
	res := &BarometerResult{SampleSize: int64(len(docs))}
	// Écho du segment demandé : l'app n'affiche « mesures Wi-Fi » que si
	// le serveur confirme avoir filtré sur ce segment.
	switch strings.ToLower(strings.TrimSpace(p.NetworkType)) {
	case "wifi":
		res.NetworkType = "wifi"
	case "mobile":
		res.NetworkType = "mobile"
	}
	family := operatorFamily(p.Operator)
	tech := strings.ToUpper(strings.TrimSpace(p.Tech))

	var all, same, sameTech, zone barometerCounts
	for _, d := range docs {
		dl, _ := strconv.ParseFloat(d.Download, 64)
		ul, _ := strconv.ParseFloat(d.Upload, 64)
		pg, _ := strconv.ParseFloat(d.Ping, 64)
		all.add(dl, ul, pg, d.StreamingScore, d.BrowsingScore, p)
		if family != "" && docOperatorFamily(d) == family {
			same.add(dl, ul, pg, d.StreamingScore, d.BrowsingScore, p)
			if tech != "" && strings.EqualFold(strings.TrimSpace(d.CellularTech), tech) {
				sameTech.add(dl, ul, pg, d.StreamingScore, d.BrowsingScore, p)
			}
		}
		if p.hasZone() && docInZone(d, p.Lat, p.Lng) {
			zone.add(dl, ul, pg, d.StreamingScore, d.BrowsingScore, p)
		}
	}

	res.DownloadPercentile, res.UploadPercentile, res.PingPercentile = all.percentiles()
	res.NationalAvgDownload = all.avgDownload()
	if p.Streaming > 0 {
		res.StreamingPercentile = all.streamingPercentile()
	}
	if p.Browsing > 0 {
		res.BrowsingPercentile = all.browsingPercentile()
	}
	// IMP-09 v2 : rang du résultat (1 = la plus rapide ; nombre de mesures
	// battues + 1) et médianes de la base, pour l'affichage sur les barres.
	res.DownloadRank, res.UploadRank, res.PingRank = all.ranks()
	res.MedianDownload, res.MedianUpload, res.MedianPing = all.medians()

	if family != "" {
		res.Operator = family
		res.OperatorSampleSize = int64(same.n)
		if same.n >= minOperatorSample {
			res.OperatorDownloadPercentile, res.OperatorUploadPercentile,
				res.OperatorPingPercentile = same.percentiles()
			res.OperatorAvgDownload = same.avgDownload()
		}
		// Sous-groupe opérateur × techno : renvoyé seulement si l'échantillon
		// suffit — sinon l'app garde le groupe opérateur entier.
		if tech != "" && sameTech.n >= minSubgroupSample {
			res.OperatorTech = tech
			res.OperatorTechSampleSize = int64(sameTech.n)
			res.OperatorTechDownloadPercentile, res.OperatorTechUploadPercentile,
				res.OperatorTechPingPercentile = sameTech.percentiles()
		}
	}
	if p.hasZone() && zone.n >= minSubgroupSample {
		res.ZoneSampleSize = int64(zone.n)
		res.ZoneDownloadPercentile, res.ZoneUploadPercentile,
			res.ZonePingPercentile = zone.percentiles()
	}

	// Tendance : les 5 derniers tests du même appareil, du plus récent au
	// plus ancien (l'app les réaffichera dans l'ordre chronologique).
	if p.DeviceID != "" {
		for _, d := range docs {
			if d.DeviceID != p.DeviceID {
				continue
			}
			dl, _ := strconv.ParseFloat(d.Download, 64)
			if dl > 0 {
				res.RecentDownloads = append(res.RecentDownloads, round2(dl))
				if len(res.RecentDownloads) == 5 {
					break
				}
			}
		}
	}
	return res
}

type barometerCounts struct {
	n                              int
	dlBetter, ulBetter, pingBetter int
	streamBetter, browseBetter     int
	streamN, browseN               int // mesures ayant testé ces usages
	dlSum                          float64
	dls, uls, pings                []float64
}

func (c *barometerCounts) add(dl, ul, pg, stream, browse float64, p BarometerParams) {
	c.n++
	c.dlSum += dl
	c.dls = append(c.dls, dl)
	c.uls = append(c.uls, ul)
	c.pings = append(c.pings, pg)
	if dl < p.Download {
		c.dlBetter++
	}
	if ul < p.Upload {
		c.ulBetter++
	}
	if pg > p.Ping {
		c.pingBetter++
	}
	// 0 = usage non testé : la mesure est exclue du centile correspondant.
	if stream > 0 {
		c.streamN++
		if stream < p.Streaming {
			c.streamBetter++
		}
	}
	if browse > 0 {
		c.browseN++
		if browse < p.Browsing {
			c.browseBetter++
		}
	}
}

func (c *barometerCounts) percentiles() (dl, ul, pg float64) {
	if c.n == 0 {
		return 0, 0, 0
	}
	n := float64(c.n)
	return round2(float64(c.dlBetter) / n * 100),
		round2(float64(c.ulBetter) / n * 100),
		round2(float64(c.pingBetter) / n * 100)
}

// streamingPercentile : part des usages de streaming testés (score > 0)
// dont le score est inférieur à celui du testeur.
func (c *barometerCounts) streamingPercentile() float64 {
	if c.streamN == 0 {
		return 0
	}
	return round2(float64(c.streamBetter) / float64(c.streamN) * 100)
}

func (c *barometerCounts) browsingPercentile() float64 {
	if c.browseN == 0 {
		return 0
	}
	return round2(float64(c.browseBetter) / float64(c.browseN) * 100)
}

func (c *barometerCounts) avgDownload() float64 {
	if c.n == 0 {
		return 0
	}
	return round2(c.dlSum / float64(c.n))
}

// ranks : le rang de chaque métrique = nombre de mesures battues + 1.
func (c *barometerCounts) ranks() (dl, ul, pg int64) {
	return int64(c.dlBetter) + 1, int64(c.ulBetter) + 1, int64(c.pingBetter) + 1
}

// medians : valeur médiane de chaque métrique dans le groupe comparé.
func (c *barometerCounts) medians() (dl, ul, pg float64) {
	return median(c.dls), median(c.uls), median(c.pings)
}

// median : médiane d'une série (copie triée, moyenne des deux valeurs
// centrales si la taille est paire). 0 si la série est vide.
func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return round2(s[n/2])
	}
	return round2((s[n/2-1] + s[n/2]) / 2)
}

// docInZone : la mesure est-elle à zoneRadiusKm km ou moins du testeur ?
// L'app doit avoir transmis une position (voir hasZone) ; une mesure sans
// position n'appartient à aucune zone.
func docInZone(d Document, lat, lng float64) bool {
	if d.Latitude == 0 && d.Longitude == 0 {
		return false
	}
	return distanceKm(lat, lng, d.Latitude, d.Longitude) <= zoneRadiusKm
}

// distanceKm : distance orthodromique simplifiée — un degré de latitude
// vaut toujours 111,32 km, un degré de longitude est corrigé de la
// cosinus de la latitude moyenne. Suffisant à l'échelle d'un quartier.
func distanceKm(lat1, lng1, lat2, lng2 float64) float64 {
	const kmPerDeg = 111.32
	dLat := (lat2 - lat1) * kmPerDeg
	mid := (lat1 + lat2) / 2 * math.Pi / 180
	dLng := (lng2 - lng1) * kmPerDeg * math.Cos(mid)
	return math.Sqrt(dLat*dLat + dLng*dLng)
}

// operatorFamily ramène les libellés d'un même opérateur mobile à un nom
// unique : « Orange BF », « Orange Burkina Faso » et « 61302 » → « Orange ».
// Un autre fournisseur (Wi-Fi, fibre) garde son libellé tel quel ; vide si
// rien n'est fourni.
func operatorFamily(s string) string {
	l := strings.ToLower(strings.TrimSpace(s))
	switch {
	case l == "":
		return ""
	case strings.Contains(l, "orange") || strings.Contains(l, "61302"):
		return "Orange"
	case strings.Contains(l, "telecel") || strings.Contains(l, "61303"):
		return "Telecel"
	case strings.Contains(l, "moov") || strings.Contains(l, "onatel") ||
		strings.Contains(l, "telmob") || strings.Contains(l, "61301"):
		return "Moov Africa"
	}
	return strings.TrimSpace(s)
}

// docOperatorFamily : l'opérateur de la SIM prime (test en données
// mobiles), sinon le fournisseur d'accès vu par l'adresse IP.
func docOperatorFamily(d Document) string {
	if f := operatorFamily(d.SimOperator); f != "" {
		return f
	}
	return operatorFamily(d.Operator)
}
