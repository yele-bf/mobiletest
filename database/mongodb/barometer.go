package mongodb

import (
	"strconv"
	"strings"
)

// minOperatorSample : en dessous, la comparaison par opérateur n'est pas
// renvoyée — un centile calculé sur trois mesures ne situe rien.
const minOperatorSample = 5

// barometerFrom calcule le baromètre (IMP-09) sur des documents déjà chargés.
// Séparé de FetchBarometer pour être testé sans base MongoDB.
//
// Centiles : part des mesures « battues » par le résultat testé (débit
// strictement inférieur ; latence strictement supérieure). Si [operator] est
// fourni, les mêmes centiles sont calculés sur les seules mesures de cet
// opérateur, reconnues par operatorFamily().
func barometerFrom(docs []Document, download, upload, ping float64, operator string) *BarometerResult {
	res := &BarometerResult{SampleSize: int64(len(docs))}
	family := operatorFamily(operator)

	var all, same barometerCounts
	for _, d := range docs {
		dl, _ := strconv.ParseFloat(d.Download, 64)
		ul, _ := strconv.ParseFloat(d.Upload, 64)
		pg, _ := strconv.ParseFloat(d.Ping, 64)
		all.add(dl, ul, pg, download, upload, ping)
		if family != "" && docOperatorFamily(d) == family {
			same.add(dl, ul, pg, download, upload, ping)
		}
	}

	res.DownloadPercentile, res.UploadPercentile, res.PingPercentile = all.percentiles()
	res.NationalAvgDownload = all.avgDownload()
	if family != "" {
		res.Operator = family
		res.OperatorSampleSize = int64(same.n)
		if same.n >= minOperatorSample {
			res.OperatorDownloadPercentile, res.OperatorUploadPercentile,
				res.OperatorPingPercentile = same.percentiles()
			res.OperatorAvgDownload = same.avgDownload()
		}
	}
	return res
}

type barometerCounts struct {
	n                              int
	dlBetter, ulBetter, pingBetter int
	dlSum                          float64
}

func (c *barometerCounts) add(dl, ul, pg, download, upload, ping float64) {
	c.n++
	c.dlSum += dl
	if dl < download {
		c.dlBetter++
	}
	if ul < upload {
		c.ulBetter++
	}
	if pg > ping {
		c.pingBetter++
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

func (c *barometerCounts) avgDownload() float64 {
	if c.n == 0 {
		return 0
	}
	return round2(c.dlSum / float64(c.n))
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
