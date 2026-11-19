package mongodb

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func qoeDoc(op string, i int) Document {
	d := Document{
		Operator:      op,
		NetworkType:   "4G",
		QoeReseau:     "mobile",
		QoeTestType:   "speedtest",
		QoeVitesse:    4,
		Download:      "1",
		QoeAge:        "20_40",
		QoeGenre:      "masculin",
		QoeAnciennete: "1_5_ans",
	}
	if i > 0 {
		d.Download = strings.TrimSpace(strings.Repeat("1", 1))
	}
	return d
}

func TestQoeStatsAggregatesAndHidesSmallSamples(t *testing.T) {
	var docs []Document
	// Orange : 12 réponses.
	for i := 0; i < 12; i++ {
		d := qoeDoc("Orange BF SA", i)
		d.Download = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"}[i]
		switch {
		case i < 3:
			d.QoeSatisfactionGlobale = "Tres_bonne"
		case i < 9:
			d.QoeSatisfactionGlobale = "Bonne"
		case i < 11:
			d.QoeSatisfactionGlobale = "Moyenne"
		default:
			d.QoeSatisfactionGlobale = "Mauvaise"
		}
		if i < 6 {
			d.QoeAmeliorations = "heures_pointe,baisser_prix"
		} else {
			d.QoeAmeliorations = "heures_pointe"
		}
		if i < 4 {
			d.QoeCoupuresVideo = "souvent"
		} else {
			d.QoeCoupuresVideo = "jamais"
		}
		if i < 3 {
			d.QoeDifficultePages = "frequemment"
		} else {
			d.QoeDifficultePages = "rarement"
		}
		docs = append(docs, d)
	}
	// Moov : 8 réponses, sous le seuil de 10 → masqué.
	for i := 0; i < 8; i++ {
		docs = append(docs, qoeDoc("Moov Africa SA", i))
	}
	// Telecel : 2 réponses.
	for i := 0; i < 2; i++ {
		docs = append(docs, qoeDoc("Telecel Faso SA", i))
	}

	st := qoeStatsFrom(docs)
	if st.Responses != 22 {
		t.Fatalf("responses = %d, attendu 22", st.Responses)
	}
	if st.Insufficient {
		t.Fatal("22 réponses ne doivent pas être insuffisantes")
	}
	if st.Satisfaction["very_satisfied"] != 3 ||
		st.Satisfaction["satisfied"] != 6 ||
		st.Satisfaction["neutral"] != 2 ||
		st.Satisfaction["unsatisfied"] != 1 {
		t.Fatalf("répartition satisfaction inattendue : %+v", st.Satisfaction)
	}
	// 9 satisfaits sur 22 → 40,9 %.
	if st.SatisfiedPct != 40.9 {
		t.Fatalf("satisfiedPct = %v, attendu 40.9", st.SatisfiedPct)
	}
	if st.AvgRating != 4 {
		t.Fatalf("avgRating = %v, attendu 4", st.AvgRating)
	}
	// 4 coupures « souvent » parmi les 12 réponses vidéo (Orange seulement).
	if st.VideoCutsPct != 33.3 {
		t.Fatalf("videoCutsPct = %v, attendu 33.3", st.VideoCutsPct)
	}
	if len(st.ByOperator) != 1 || st.ByOperator[0].Operator != "Orange BF SA" {
		t.Fatalf("seul Orange doit dépasser le seuil : %+v", st.ByOperator)
	}
	orange := st.ByOperator[0]
	if orange.Responses != 12 {
		t.Fatalf("responses Orange = %d", orange.Responses)
	}
	if orange.AvgSatisfaction != 3.92 {
		t.Fatalf("satisfaction moyenne Orange = %v, attendu 3.92", orange.AvgSatisfaction)
	}
	if orange.UnsatisfiedPct != 8.3 {
		t.Fatalf("insatisfaits Orange = %v, attendu 8.3", orange.UnsatisfiedPct)
	}
	if orange.MedianDownload != 6.5 {
		t.Fatalf("médiane débit Orange = %v, attendu 6.5", orange.MedianDownload)
	}
	if orange.VideoCutsPct != 33.3 {
		t.Fatalf("coupures vidéo Orange = %v, attendu 33.3", orange.VideoCutsPct)
	}
	// 3 « fréquemment » sur 12 réponses pages → 25 %.
	if orange.PagesOftenPct != 25 {
		t.Fatalf("pages souvent Orange = %v, attendu 25", orange.PagesOftenPct)
	}
	if len(st.AmeliorationsMobile) < 2 {
		t.Fatalf("améliorations mobiles attendues : %+v", st.AmeliorationsMobile)
	}
	if st.AmeliorationsMobile[0].Key != "heures_pointe" ||
		st.AmeliorationsMobile[0].Count != 12 ||
		st.AmeliorationsMobile[0].Pct != 54.5 {
		t.Fatalf("première amélioration = %+v", st.AmeliorationsMobile[0])
	}
	if len(st.Points) != 22 {
		t.Fatalf("points du nuage = %d, attendu 22", len(st.Points))
	}
}

func TestQoeStatsInsufficientUnderTen(t *testing.T) {
	docs := []Document{
		{Operator: "Orange BF SA", QoeVitesse: 3},
		{Operator: "Orange BF SA", QoeVitesse: 4},
	}
	st := qoeStatsFrom(docs)
	if !st.Insufficient {
		t.Fatal("2 réponses doivent être marquées insuffisantes")
	}
	if len(st.ByOperator) != 0 {
		t.Fatalf("aucun opérateur ne doit être publié : %+v", st.ByOperator)
	}
}

func TestQoeRatingFallsBackOnSatisfaction(t *testing.T) {
	d := Document{QoeSatisfactionGlobale: "Bonne"}
	if got := docRating(&d); got != 4 {
		t.Fatalf("docRating = %d, attendu 4", got)
	}
	d = Document{QoeSat: "very_unsatisfied"}
	if got := docRating(&d); got != 1 {
		t.Fatalf("docRating (ancien) = %d, attendu 1", got)
	}
	d = Document{QoeVitesse: 5, QoeSatisfactionGlobale: "Mauvaise"}
	if got := docRating(&d); got != 5 {
		t.Fatalf("les étoiles doivent primer : %d", got)
	}
}

func TestDocReseauFallback(t *testing.T) {
	if got := docReseau(&Document{NetworkType: "Wi-Fi 5"}); got != "fixe" {
		t.Fatalf("Wi-Fi = %s", got)
	}
	if got := docReseau(&Document{NetworkType: "4G"}); got != "mobile" {
		t.Fatalf("4G = %s", got)
	}
	if got := docReseau(&Document{QoeReseau: "fixe", NetworkType: "4G"}); got != "fixe" {
		t.Fatalf("qoe_reseau doit primer : %s", got)
	}
}

func TestQoeOperatorLabelsMerge(t *testing.T) {
	var docs []Document
	// 6 « Orange BF » + 6 « Orange Burkina Faso » : un seul groupe ARCEP de 12.
	for i := 0; i < 6; i++ {
		docs = append(docs, Document{Operator: "Orange BF", SimOperator: "Orange BF",
			QoeReseau: "mobile", QoeVitesse: 4})
		docs = append(docs, Document{Operator: "Orange Burkina Faso",
			QoeReseau: "mobile", QoeVitesse: 3})
	}
	st := qoeStatsFrom(docs)
	if len(st.ByOperator) != 1 {
		t.Fatalf("un seul opérateur attendu : %+v", st.ByOperator)
	}
	got := st.ByOperator[0]
	if got.Operator != "Orange BF SA" || got.Responses != 12 {
		t.Fatalf("groupe Orange = %+v", got)
	}
	if got.AvgRating != 3.5 {
		t.Fatalf("note moyenne = %v, attendu 3.5", got.AvgRating)
	}

	// En fixe, c'est le fournisseur vu par l'IP qui compte, pas la SIM.
	d := Document{QoeReseau: "fixe", Operator: "Canalbox", SimOperator: "Orange BF"}
	if label := docOperator(&d); label != "Canalbox" {
		t.Fatalf("opérateur fixe = %q, attendu Canalbox", label)
	}
}

func TestQoeFilterMatch(t *testing.T) {
	d := &Document{
		Operator:     "Orange Burkina Faso",
		SimOperator:  "Orange BF",
		CellularTech: "4G",
		QoeReseau:    "mobile",
	}
	f := QoeFilter{Reseau: "mobile", Operator: "orange", Tech: "4g"}
	if !f.match(d) {
		t.Fatal("le filtre devrait correspondre (casse et sous-chaîne)")
	}
	if (QoeFilter{Reseau: "fixe"}).match(d) {
		t.Fatal("un test mobile ne doit pas passer le filtre fixe")
	}
	if (QoeFilter{Tech: "5G"}).match(d) {
		t.Fatal("un test 4G ne doit pas passer le filtre 5G")
	}
}

func TestQoePeriodStart(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	start, ok := (QoeFilter{Period: "month"}).periodStart(now)
	if !ok || !start.Equal(now.Add(-30*24*time.Hour)) {
		t.Fatalf("période mois = %v, %v", start, ok)
	}
	if _, ok := (QoeFilter{Period: "tout"}).periodStart(now); ok {
		t.Fatal("« tout » ne doit pas borner la période")
	}
}

// Les commentaires libres et le champ « Autre » ne doivent jamais sortir de
// l'API publique.
func TestDocumentQoePrivateFieldsHiddenInJSON(t *testing.T) {
	b, err := json.Marshal(Document{
		QoeCommentaire:         "mon numéro 70 00 00 00",
		QoeAmeliorationsAutre:  "détail personnel",
		QoeAge:                 "20_40",
		QoeSatisfactionGlobale: "Bonne",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "mon numéro") || strings.Contains(s, "détail personnel") {
		t.Fatalf("donnée privée exposée : %s", s)
	}
	if !strings.Contains(s, "20_40") || !strings.Contains(s, "Bonne") {
		t.Fatalf("les champs publics manquent : %s", s)
	}
}
