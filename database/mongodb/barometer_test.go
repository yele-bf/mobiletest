package mongodb

import "testing"

func doc(op, sim, dl, ul, ping string) Document {
	return Document{Operator: op, SimOperator: sim, Download: dl, Upload: ul, Ping: ping}
}

// IMP-09 : centiles nationaux et parmi les clients du même opérateur.
func TestBarometerFromParOperateur(t *testing.T) {
	docs := []Document{
		// 6 mesures Orange (libellés différents), débits 5 à 30 Mb/s.
		doc("Orange Burkina Faso", "Orange BF", "5", "1", "80"),
		doc("ONATEL", "Orange BF", "10", "2", "70"), // SIM Orange prime sur le FAI
		doc("Orange Burkina Faso", "", "15", "3", "60"),
		doc("", "61302", "20", "4", "50"),
		doc("Orange Burkina Faso", "Orange", "25", "5", "40"),
		doc("Orange Burkina Faso", "Orange", "30", "6", "30"),
		// 4 mesures d'autres opérateurs, plus rapides.
		doc("Telecel Faso", "Telecel", "40", "8", "20"),
		doc("Moov Africa", "Moov Africa", "50", "9", "20"),
		doc("Canalbox", "", "60", "10", "10"),
		doc("Canalbox", "", "70", "11", "10"),
	}

	res := barometerFrom(docs, 22, 4.5, 45, "Orange BF")

	if res.SampleSize != 10 {
		t.Fatalf("échantillon %d, attendu 10", res.SampleSize)
	}
	// National : 22 Mb/s bat 4 mesures sur 10 (5, 10, 15, 20).
	if res.DownloadPercentile != 40 {
		t.Errorf("centile national download %v, attendu 40", res.DownloadPercentile)
	}
	if res.NationalAvgDownload != 32.5 {
		t.Errorf("moyenne nationale %v, attendu 32.5", res.NationalAvgDownload)
	}
	if res.Operator != "Orange" || res.OperatorSampleSize != 6 {
		t.Fatalf("opérateur %q / %d mesures, attendu Orange / 6", res.Operator, res.OperatorSampleSize)
	}
	// Parmi Orange : 22 bat 4 mesures sur 6 ; upload 4,5 bat 4 sur 6 ;
	// latence 45 ms meilleure que 4 sur 6 (80, 70, 60, 50).
	if res.OperatorDownloadPercentile != 66.67 {
		t.Errorf("centile Orange download %v, attendu 66.67", res.OperatorDownloadPercentile)
	}
	if res.OperatorUploadPercentile != 66.67 {
		t.Errorf("centile Orange upload %v, attendu 66.67", res.OperatorUploadPercentile)
	}
	if res.OperatorPingPercentile != 66.67 {
		t.Errorf("centile Orange latence %v, attendu 66.67", res.OperatorPingPercentile)
	}
	if res.OperatorAvgDownload != 17.5 {
		t.Errorf("moyenne Orange %v, attendu 17.5", res.OperatorAvgDownload)
	}
}

// Sans opérateur, ou avec trop peu de mesures, pas de comparaison par
// opérateur (mais le centile national reste calculé).
func TestBarometerFromSansComparaisonOperateur(t *testing.T) {
	docs := []Document{
		doc("Telecel Faso", "Telecel", "10", "2", "50"),
		doc("Telecel Faso", "Telecel", "20", "4", "40"),
		doc("Orange Burkina Faso", "Orange", "30", "6", "30"),
	}

	sans := barometerFrom(docs, 15, 3, 45, "")
	if sans.Operator != "" || sans.OperatorSampleSize != 0 {
		t.Errorf("opérateur %q / %d, attendu aucun", sans.Operator, sans.OperatorSampleSize)
	}
	if sans.DownloadPercentile != 33.33 {
		t.Errorf("centile national %v, attendu 33.33", sans.DownloadPercentile)
	}

	peu := barometerFrom(docs, 15, 3, 45, "Telecel")
	if peu.OperatorSampleSize != 2 || peu.OperatorDownloadPercentile != 0 || peu.OperatorAvgDownload != 0 {
		t.Errorf("2 mesures Telecel : %+v, attendu échantillon 2 sans centile", peu)
	}
}

func TestOperatorFamily(t *testing.T) {
	cases := map[string]string{
		"Orange BF": "Orange", "Orange Burkina Faso": "Orange", "61302": "Orange",
		"TELECEL FASO": "Telecel", "Telmob (Onatel)": "Moov Africa",
		"Moov Africa": "Moov Africa", "Canalbox": "Canalbox", " ": "",
	}
	for in, want := range cases {
		if got := operatorFamily(in); got != want {
			t.Errorf("operatorFamily(%q) = %q, attendu %q", in, got, want)
		}
	}
}
