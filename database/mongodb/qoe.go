package mongodb

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MinQoeSample : en dessous, les agrégats par opérateur sont masqués — même
// garde-fou que le baromètre, un pourcentage calculé sur trois réponses ne
// situe rien.
const MinQoeSample = 10

// ── Filtres ──────────────────────────────────────────────────────────────────

// QoeFilter reprend les filtres de l'onglet « Qualité perçue » : réseau
// (mobile/fixe), opérateur, technologie et période.
type QoeFilter struct {
	Reseau   string // "mobile", "fixe" ou vide (tous)
	Operator string // sous-chaîne, insensible à la casse
	Tech     string // "2G"…"5G" (mobile uniquement)
	Period   string // hour, day, week, month, year, all
	From, To string // YYYY-MM-DD, prioritaires sur Period
}

// periodStart traduit les libellés de période du dashboard en fenêtre
// glissante. « tout »/vide = pas de borne.
func (f QoeFilter) periodStart(now time.Time) (time.Time, bool) {
	switch strings.ToLower(strings.TrimSpace(f.Period)) {
	case "hour", "heure":
		return now.Add(-time.Hour), true
	case "day", "jour", "jours":
		return now.Add(-24 * time.Hour), true
	case "week", "semaine", "semaines":
		return now.Add(-7 * 24 * time.Hour), true
	case "month", "mois":
		return now.Add(-30 * 24 * time.Hour), true
	case "year", "annee", "annees", "année", "années":
		return now.Add(-365 * 24 * time.Hour), true
	}
	return time.Time{}, false
}

// bsonFilter ne retient que les mesures actives ayant une évaluation QoE.
func (f QoeFilter) bsonFilter(now time.Time) bson.M {
	filter := withoutPassive(bson.M{
		"$or": bson.A{
			bson.M{"qoe_rating": bson.M{"$gt": 0}},
			bson.M{"qoe_test_type": bson.M{"$ne": ""}},
		},
	})
	timeFilter := bson.M{}
	if start, ok := f.periodStart(now); ok {
		timeFilter["$gte"] = start
	}
	if f.From != "" {
		if t, err := time.Parse("2006-01-02", f.From); err == nil {
			timeFilter["$gte"] = t
		}
	}
	if f.To != "" {
		if t, err := time.Parse("2006-01-02", f.To); err == nil {
			timeFilter["$lte"] = t.Add(24 * time.Hour)
		}
	}
	if len(timeFilter) > 0 {
		filter["timestamp"] = timeFilter
	}
	return filter
}

// docReseau : réseau déclaré par l'app récente, sinon déduit de network_type.
// Les anciennes mesures n'ont pas qoe_reseau.
func docReseau(d *Document) string {
	if d.QoeReseau == "mobile" || d.QoeReseau == "fixe" {
		return d.QoeReseau
	}
	nt := strings.NewReplacer("-", "", " ", "", "_", "").Replace(strings.ToUpper(d.NetworkType))
	if strings.Contains(nt, "WIFI") || strings.Contains(nt, "WLAN") {
		return "fixe"
	}
	return "mobile"
}

// qoeOperatorLabel traduit les variantes de libellé en nom ARCEP lisible.
// Les regroupements reprennent operatorFamily (baromètre) : « Orange BF »,
// « Orange Burkina Faso » et « 61302 » désignent le même opérateur.
func qoeOperatorLabel(name string) string {
	switch operatorFamily(name) {
	case "Orange":
		return "Orange BF SA"
	case "Moov Africa":
		return "Moov Africa SA"
	case "Telecel":
		return "Telecel Faso SA"
	}
	return strings.TrimSpace(name)
}

// docOperator : l'opérateur qui compte pour un test. En mobile, la SIM prime
// (l'IP peut sortir par un FAI fixe) ; en fixe, le fournisseur vu par l'IP
// prime (la SIM n'a rien à voir avec le Wi-Fi utilisé).
func docOperator(d *Document) string {
	if docReseau(d) == "mobile" {
		if label := qoeOperatorLabel(d.SimOperator); label != "" {
			return label
		}
	}
	if label := qoeOperatorLabel(d.Operator); label != "" {
		return label
	}
	return qoeOperatorLabel(d.SimOperator)
}

// match applique en Go les filtres que les anciens documents ne peuvent pas
// porter (qoe_reseau absent). Le volume reste celui des réponses QoE.
func (f QoeFilter) match(d *Document) bool {
	if f.Reseau != "" && docReseau(d) != f.Reseau {
		return false
	}
	if f.Operator != "" {
		needle := strings.ToLower(strings.TrimSpace(f.Operator))
		haystack := strings.ToLower(d.Operator + " " + d.SimOperator + " " + docOperator(d))
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	if f.Tech != "" && !strings.EqualFold(strings.TrimSpace(d.CellularTech), f.Tech) {
		return false
	}
	return true
}

// ── Conversions QoE ──────────────────────────────────────────────────────────

var satisfactionToRating = map[string]int{
	"Tres_bonne":    5,
	"Bonne":         4,
	"Moyenne":       3,
	"Mauvaise":      2,
	"Tres_mauvaise": 1,
}

var satisfactionToCode = map[string]string{
	"Tres_bonne":    "very_satisfied",
	"Bonne":         "satisfied",
	"Moyenne":       "neutral",
	"Mauvaise":      "unsatisfied",
	"Tres_mauvaise": "very_unsatisfied",
}

var satisfactionScores = map[string]int{
	"very_satisfied":   5,
	"satisfied":        4,
	"neutral":          3,
	"unsatisfied":      2,
	"very_unsatisfied": 1,
}

// docSatisfactionCode : satisfaction globale du formulaire, avec repli sur le
// code historique (anciennes mesures : 2 questions seulement).
func docSatisfactionCode(d *Document) (string, bool) {
	if code, ok := satisfactionToCode[d.QoeSatisfactionGlobale]; ok {
		return code, true
	}
	if d.QoeSat != "" {
		return d.QoeSat, true
	}
	return "", false
}

// docRating : étoiles de vitesse si la question a été posée, sinon conversion
// de la satisfaction globale.
func docRating(d *Document) int {
	if d.QoeVitesse > 0 && d.QoeVitesse <= 5 {
		return d.QoeVitesse
	}
	if r, ok := satisfactionToRating[d.QoeSatisfactionGlobale]; ok {
		return r
	}
	if score, ok := satisfactionScores[d.QoeSat]; ok {
		return score
	}
	return 0
}

func isOften(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "frequemment" || v == "souvent" ||
		v == "fréquemment" || v == "Fréquemment"
}

// ── Résultats ────────────────────────────────────────────────────────────────

type QoeOperatorStat struct {
	Operator        string  `json:"operator"`
	Responses       int     `json:"responses"`
	AvgSatisfaction float64 `json:"avgSatisfaction"` // sur 5
	UnsatisfiedPct  float64 `json:"unsatisfiedPct"`
	AvgRating       float64 `json:"avgRating"` // sur 5
	PagesOftenPct   float64 `json:"pagesOftenPct"`
	VideoCutsPct    float64 `json:"videoCutsPct"`
	MedianDownload  float64 `json:"medianDownload"` // Mb/s
}

type QoeImprovement struct {
	Key   string  `json:"key"`
	Count int     `json:"count"`
	Pct   float64 `json:"pct"`
}

type QoeProfile struct {
	Age        map[string]int `json:"age"`
	Genre      map[string]int `json:"genre"`
	Anciennete map[string]int `json:"anciennete"`
}

type QoeScatterPoint struct {
	Rating   int     `json:"rating"`
	Download float64 `json:"download"`
	Operator string  `json:"operator"`
}

type QoeStats struct {
	Responses int `json:"responses"`
	// TestsTotal : tous les tests actifs de la même période et du même réseau,
	// évalués ou non. Sert à calculer la part des tests qui ont reçu une
	// évaluation (« 61 % des tests de la période »).
	TestsTotal   int     `json:"testsTotal"`
	SatisfiedPct float64 `json:"satisfiedPct"`
	// VideoAnswered : nombre de réponses à la question « coupures vidéo ».
	// Sans lui, 0 % serait ambigu (aucune coupure déclarée ou question non posée).
	VideoAnswered       int               `json:"videoAnswered"`
	AvgRating           float64           `json:"avgRating"`
	VideoCutsPct        float64           `json:"videoCutsPct"`
	Insufficient        bool              `json:"insufficient"`
	MinSample           int               `json:"minSample"`
	Satisfaction        map[string]int    `json:"satisfaction"`
	ByOperator          []QoeOperatorStat `json:"byOperator"`
	Profile             QoeProfile        `json:"profile"`
	AmeliorationsMobile []QoeImprovement  `json:"ameliorationsMobile"`
	AmeliorationsFixe   []QoeImprovement  `json:"ameliorationsFixe"`
	// Nombre de répondants de chaque réseau : le bloc « Améliorations »
	// affiche les deux colonnes avec leur base de calcul.
	AmeliorationsMobileN int               `json:"ameliorationsMobileN"`
	AmeliorationsFixeN   int               `json:"ameliorationsFixeN"`
	Points               []QoeScatterPoint `json:"points"`
}

type qoeOperatorAgg struct {
	count, satN, satSum, unsat int
	ratingN                    int
	ratingSum                  float64
	pagesN, pagesOften         int
	videoN, videoOften         int
	downloads                  []float64
}

func newQoeStats() *QoeStats {
	return &QoeStats{
		MinSample: MinQoeSample,
		Satisfaction: map[string]int{
			"very_satisfied":   0,
			"satisfied":        0,
			"neutral":          0,
			"unsatisfied":      0,
			"very_unsatisfied": 0,
		},
		Profile: QoeProfile{
			Age:        map[string]int{},
			Genre:      map[string]int{},
			Anciennete: map[string]int{},
		},
		ByOperator:          []QoeOperatorStat{},
		AmeliorationsMobile: []QoeImprovement{},
		AmeliorationsFixe:   []QoeImprovement{},
		Points:              []QoeScatterPoint{},
	}
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

// qoeStatsFrom agrège des documents déjà chargés et filtrés. Séparé de
// FetchQoeStats pour être testé sans MongoDB.
func qoeStatsFrom(docs []Document) *QoeStats {
	st := newQoeStats()
	var satisfied, videoAnswered, videoCuts int
	var ratingSum float64
	var ratingN int
	perOp := map[string]*qoeOperatorAgg{}
	improveMobile := map[string]int{}
	improveFixe := map[string]int{}
	mobileN, fixeN := 0, 0

	for i := range docs {
		d := &docs[i]
		st.Responses++

		// Satisfaction globale.
		if code, ok := docSatisfactionCode(d); ok {
			st.Satisfaction[code]++
			if code == "very_satisfied" || code == "satisfied" {
				satisfied++
			}
		}

		// Note de vitesse (ou repli satisfaction).
		rating := docRating(d)
		if rating > 0 {
			ratingSum += float64(rating)
			ratingN++
		}

		// Coupures vidéo déclarées « souvent/fréquemment ».
		if d.QoeCoupuresVideo != "" {
			videoAnswered++
			if isOften(d.QoeCoupuresVideo) {
				videoCuts++
			}
		}

		// Profil des répondants.
		if d.QoeAge != "" {
			st.Profile.Age[d.QoeAge]++
		}
		if d.QoeGenre != "" {
			st.Profile.Genre[d.QoeGenre]++
		}
		if d.QoeAnciennete != "" {
			st.Profile.Anciennete[d.QoeAnciennete]++
		}

		// Améliorations prioritaires, séparées mobile / fixe.
		reseau := docReseau(d)
		if reseau == "fixe" {
			fixeN++
		} else {
			mobileN++
		}
		for _, key := range strings.Split(d.QoeAmeliorations, ",") {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if reseau == "fixe" {
				improveFixe[key]++
			} else {
				improveMobile[key]++
			}
		}

		// Agrégat par opérateur (libellé ARCEP canonique).
		op := docOperator(d)
		if op == "" {
			op = "Inconnu"
		}
		oa := perOp[op]
		if oa == nil {
			oa = &qoeOperatorAgg{}
			perOp[op] = oa
		}
		oa.count++
		if code, ok := docSatisfactionCode(d); ok {
			oa.satN++
			oa.satSum += satisfactionScores[code]
			if code == "unsatisfied" || code == "very_unsatisfied" {
				oa.unsat++
			}
		}
		if rating > 0 {
			oa.ratingN++
			oa.ratingSum += float64(rating)
		}
		if d.QoeDifficultePages != "" {
			oa.pagesN++
			if isOften(d.QoeDifficultePages) {
				oa.pagesOften++
			}
		}
		if d.QoeCoupuresVideo != "" {
			oa.videoN++
			if isOften(d.QoeCoupuresVideo) {
				oa.videoOften++
			}
		}
		dl, _ := strconv.ParseFloat(d.Download, 64)
		if dl > 0 {
			oa.downloads = append(oa.downloads, dl)
			if rating > 0 && len(st.Points) < 1000 {
				st.Points = append(st.Points, QoeScatterPoint{
					Rating:   rating,
					Download: round2(dl),
					Operator: op,
				})
			}
		}
	}

	if st.Responses > 0 {
		st.SatisfiedPct = round1(100 * float64(satisfied) / float64(st.Responses))
		st.VideoAnswered = videoAnswered
		if videoAnswered > 0 {
			st.VideoCutsPct = round1(100 * float64(videoCuts) / float64(videoAnswered))
		}
	}
	if ratingN > 0 {
		st.AvgRating = round2(ratingSum / float64(ratingN))
	}
	st.Insufficient = st.Responses < MinQoeSample

	// Opérateurs : groupes sous le seuil masqués.
	for op, oa := range perOp {
		if oa.count < MinQoeSample {
			continue
		}
		stat := QoeOperatorStat{Operator: op, Responses: oa.count}
		if oa.satN > 0 {
			stat.AvgSatisfaction = round2(float64(oa.satSum) / float64(oa.satN))
			stat.UnsatisfiedPct = round1(100 * float64(oa.unsat) / float64(oa.satN))
		}
		if oa.ratingN > 0 {
			stat.AvgRating = round2(oa.ratingSum / float64(oa.ratingN))
		}
		if oa.pagesN > 0 {
			stat.PagesOftenPct = round1(100 * float64(oa.pagesOften) / float64(oa.pagesN))
		}
		if oa.videoN > 0 {
			stat.VideoCutsPct = round1(100 * float64(oa.videoOften) / float64(oa.videoN))
		}
		stat.MedianDownload = median(oa.downloads)
		st.ByOperator = append(st.ByOperator, stat)
	}
	sort.Slice(st.ByOperator, func(i, j int) bool {
		return st.ByOperator[i].Responses > st.ByOperator[j].Responses
	})

	st.AmeliorationsMobile = qoeImprovements(improveMobile, mobileN)
	st.AmeliorationsFixe = qoeImprovements(improveFixe, fixeN)
	return st
}

func qoeImprovements(counts map[string]int, respondents int) []QoeImprovement {
	out := make([]QoeImprovement, 0, len(counts))
	for key, count := range counts {
		pct := 0.0
		if respondents > 0 {
			pct = round1(100 * float64(count) / float64(respondents))
		}
		out = append(out, QoeImprovement{Key: key, Count: count, Pct: pct})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Key < out[j].Key
		}
		return out[i].Count > out[j].Count
	})
	return out
}

// qoeImprovementsFrom recalcule les deux listes d'améliorations sur des
// documents filtrés par période / opérateur / techno, sans filtrer le réseau :
// le dashboard montre toujours les colonnes mobile ET fixe, même quand la vue
// courante est limitée à l'un des deux.
func qoeImprovementsFrom(docs []Document) (mobile, fixe []QoeImprovement, mobileN, fixeN int) {
	mobileCounts := map[string]int{}
	fixeCounts := map[string]int{}
	for i := range docs {
		d := &docs[i]
		reseau := docReseau(d)
		if reseau == "fixe" {
			fixeN++
		} else {
			mobileN++
		}
		for _, key := range strings.Split(d.QoeAmeliorations, ",") {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if reseau == "fixe" {
				fixeCounts[key]++
			} else {
				mobileCounts[key]++
			}
		}
	}
	return qoeImprovements(mobileCounts, mobileN), qoeImprovements(fixeCounts, fixeN), mobileN, fixeN
}

// ── Accès MongoDB ────────────────────────────────────────────────────────────

func (m *MongoDB) FetchQoeStats(f QoeFilter) (*QoeStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(20000)
	cursor, err := m.collection.Find(ctx, f.bsonFilter(time.Now()), opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []Document
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	filtered := make([]Document, 0, len(docs))
	for i := range docs {
		if f.match(&docs[i]) {
			filtered = append(filtered, docs[i])
		}
	}
	st := qoeStatsFrom(filtered)
	// Améliorations : les deux réseaux sont toujours présentés côte à côte.
	// Le filtre réseau ne s'applique donc pas à ce bloc (mais opérateur,
	// techno et période, oui).
	allForImprovements := make([]Document, 0, len(docs))
	noReseau := f
	noReseau.Reseau = ""
	for i := range docs {
		if noReseau.match(&docs[i]) {
			allForImprovements = append(allForImprovements, docs[i])
		}
	}
	st.AmeliorationsMobile, st.AmeliorationsFixe, st.AmeliorationsMobileN, st.AmeliorationsFixeN =
		qoeImprovementsFrom(allForImprovements)
	// Volume total de tests de la même période : la part des tests évalués se
	// lit ainsi « 61 % des tests de la période », sans mélanger les périodes.
	if total, err := m.countTests(f, time.Now()); err == nil {
		st.TestsTotal = total
	}
	return st, nil
}

// countTests compte les tests actifs de la période, évalués ou non. Le réseau,
// l'opérateur et la technologie sont appliqués en Go : les anciens documents
// n'ont pas qoe_reseau et le repli sur network_type ne s'exprime pas en BSON.
func (m *MongoDB) countTests(f QoeFilter, now time.Time) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	filter := withoutPassive(bson.M{})
	timeFilter := bson.M{}
	if start, ok := f.periodStart(now); ok {
		timeFilter["$gte"] = start
	}
	if f.From != "" {
		if t, err := time.Parse("2006-01-02", f.From); err == nil {
			timeFilter["$gte"] = t
		}
	}
	if f.To != "" {
		if t, err := time.Parse("2006-01-02", f.To); err == nil {
			timeFilter["$lte"] = t.Add(24 * time.Hour)
		}
	}
	if len(timeFilter) > 0 {
		filter["timestamp"] = timeFilter
	}
	opts := options.Find().
		SetProjection(bson.M{
			"network_type": 1, "cellular_tech": 1,
			"operator": 1, "sim_operator": 1, "qoe_reseau": 1,
		}).
		SetLimit(100000)
	cursor, err := m.collection.Find(ctx, filter, opts)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	total := 0
	for cursor.Next(ctx) {
		var d Document
		if err := cursor.Decode(&d); err != nil {
			continue
		}
		if f.match(&d) {
			total++
		}
	}
	return total, cursor.Err()
}

// FetchQoeMapPoints renvoie les mesures évaluées géolocalisées, pour la
// couche « Qualité perçue » de l'onglet Carte.
func (m *MongoDB) FetchQoeMapPoints(f QoeFilter) ([]MapPoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	filter := f.bsonFilter(time.Now())
	filter["latitude"] = bson.M{"$ne": 0}
	filter["longitude"] = bson.M{"$ne": 0}
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(1000)
	cursor, err := m.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []Document
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	points := make([]MapPoint, 0, len(docs))
	for i := range docs {
		doc := &docs[i]
		if !f.match(doc) {
			continue
		}
		consolidateSignal(doc)
		dl, _ := strconv.ParseFloat(doc.Download, 64)
		ul, _ := strconv.ParseFloat(doc.Upload, 64)
		ping, _ := strconv.ParseFloat(doc.Ping, 64)
		sat, _ := docSatisfactionCode(doc)
		points = append(points, MapPoint{
			Lat:                    doc.Latitude,
			Lng:                    doc.Longitude,
			Operator:               doc.Operator,
			QoeOperator:            docOperator(doc),
			NetworkType:            doc.NetworkType,
			SimOperator:            doc.SimOperator,
			CellularTech:           doc.CellularTech,
			Download:               round2(dl),
			Upload:                 round2(ul),
			Ping:                   round2(ping),
			Location:               doc.Location,
			Timestamp:              doc.Timestamp.Format("02/01/2006 15:04"),
			StreamingScore:         round2(doc.StreamingScore),
			StreamingMaxResolution: doc.StreamingMaxResolution,
			StreamingRebufferRatio: round2(doc.StreamingRebufferRatio),
			BrowsingAvgLoadMs:      round2(doc.BrowsingAvgLoadMs),
			BrowsingScore:          round2(doc.BrowsingScore),
			PacketLoss:             doc.PacketLoss,
			Rsrp:                   doc.Rsrp,
			Rsrq:                   doc.Rsrq,
			Rssi:                   doc.Rssi,
			QoeTestType:            doc.QoeTestType,
			QoeReseau:              docReseau(doc),
			QoeSatisfaction:        sat,
			QoeRating:              docRating(doc),
			QoeVitesse:             doc.QoeVitesse,
			QoeCoupuresVideo:       doc.QoeCoupuresVideo,
		})
	}
	return points, nil
}
