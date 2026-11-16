package mongodb

import (
	"strconv"
	"testing"
)

func doc(op, sim, dl, ul, ping string) Document {
	return Document{Operator: op, SimOperator: sim, Download: dl, Upload: ul, Ping: ping}
}

// IMP-09 : centiles nationaux et parmi les clients du même opérateur.
// IMP-09 v2 : rangs, médianes et tendance sur les 5 derniers tests.
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

	res := barometerFrom(docs, BarometerParams{Download: 22, Upload: 4.5, Ping: 45, Operator: "Orange BF"})

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
	// Rangs : 4 mesures battues + 1, sur les trois métriques.
	if res.DownloadRank != 5 || res.UploadRank != 5 || res.PingRank != 5 {
		t.Errorf("rangs %d/%d/%d, attendu 5/5/5", res.DownloadRank, res.UploadRank, res.PingRank)
	}
	// Médianes nationales : dl trié 5..30,40,50,60,70 → (25+30)/2 ;
	// ul 1..11 → (5+6)/2 ; ping 10..80 → (30+40)/2.
	if res.MedianDownload != 27.5 || res.MedianUpload != 5.5 || res.MedianPing != 35 {
		t.Errorf("médianes %v/%v/%v, attendu 27.5/5.5/35",
			res.MedianDownload, res.MedianUpload, res.MedianPing)
	}
}

// IMP-09 v2 : sans appareil connu, pas de tendance ; avec un appareil
// présent dans la base, les 5 derniers tests (du plus récent au plus
// ancien, les docs sont triés ainsi).
func TestBarometerFromTendance(t *testing.T) {
	d1 := "d1"
	docs := []Document{
		// Appareil d1, du plus récent au plus ancien : 20, 18, 16, 14, 12, 10.
		{DeviceID: d1, Download: "20"},
		{DeviceID: d1, Download: "18"},
		{DeviceID: d1, Download: "16"},
		{DeviceID: d1, Download: "14"},
		{DeviceID: d1, Download: "12"},
		{DeviceID: d1, Download: "10"},
		// Un autre appareil, ignoré.
		{DeviceID: "d2", Download: "99"},
	}

	res := barometerFrom(docs, BarometerParams{Download: 15, DeviceID: d1})
	want := []float64{20, 18, 16, 14, 12}
	if len(res.RecentDownloads) != 5 {
		t.Fatalf("tendance %v, attendu 5 valeurs", res.RecentDownloads)
	}
	for i, w := range want {
		if res.RecentDownloads[i] != w {
			t.Errorf("tendance[%d] = %v, attendu %v", i, res.RecentDownloads[i], w)
		}
	}

	inconnu := barometerFrom(docs, BarometerParams{Download: 15, DeviceID: "x"})
	if len(inconnu.RecentDownloads) != 0 {
		t.Errorf("appareil inconnu : tendance %v, attendu vide", inconnu.RecentDownloads)
	}
	sans := barometerFrom(docs, BarometerParams{Download: 15})
	if len(sans.RecentDownloads) != 0 {
		t.Errorf("sans deviceID : tendance %v, attendu vide", sans.RecentDownloads)
	}
}

// IMP-09 v2 : scores de streaming et de navigation. Une mesure sans test
// (score 0) est exclue du centile correspondant ; un testeur qui n'a pas
// fait l'usage ne reçoit pas le centile.
func TestBarometerFromStreamingNavigation(t *testing.T) {
	docs := []Document{
		{Download: "10", StreamingScore: 50, BrowsingScore: 40},
		{Download: "20", StreamingScore: 70, BrowsingScore: 0}, // pas de navigation
		{Download: "30", StreamingScore: 90, BrowsingScore: 80},
		{Download: "40", StreamingScore: 0, BrowsingScore: 95}, // pas de streaming
		{Download: "50", StreamingScore: 100, BrowsingScore: 100},
	}

	res := barometerFrom(docs, BarometerParams{Download: 25, Streaming: 75, Browsing: 85})
	if res.StreamingPercentile != 50 {
		t.Errorf("centile streaming %v, attendu 50 (bat 50 et 70 sur 4 tests)", res.StreamingPercentile)
	}
	if res.BrowsingPercentile != 50 {
		t.Errorf("centile navigation %v, attendu 50 (bat 40 et 80 sur 4 tests)", res.BrowsingPercentile)
	}

	// Testeur sans test de streaming : pas de centile renvoyé.
	sans := barometerFrom(docs, BarometerParams{Download: 25, Browsing: 85})
	if sans.StreamingPercentile != 0 {
		t.Errorf("centile streaming %v, attendu 0 (usage non testé)", sans.StreamingPercentile)
	}
	if sans.BrowsingPercentile != 50 {
		t.Errorf("centile navigation %v, attendu 50", sans.BrowsingPercentile)
	}
}

// IMP-09 v2 : sous-groupe opérateur × techno, renvoyé à partir de
// minSubgroupSample (10) mesures seulement.
func TestBarometerFromTechno(t *testing.T) {
	var docs []Document
	// 12 mesures Orange en 4G (dl 1 à 12), 3 Orange en 3G.
	for i := 1; i <= 12; i++ {
		d := doc("Orange BF", "Orange", itoa(i), "1", "50")
		d.CellularTech = "4G"
		docs = append(docs, d)
	}
	for i := 1; i <= 3; i++ {
		d := doc("Orange BF", "Orange", itoa(i+20), "1", "50")
		d.CellularTech = "3G"
		docs = append(docs, d)
	}

	res := barometerFrom(docs, BarometerParams{
		Download: 6.5, Operator: "Orange BF", Tech: "4G",
	})
	// Parmi Orange : dls 1..12 et 21..23 ; 6,5 bat 6 mesures sur 15.
	if res.OperatorSampleSize != 15 || res.OperatorDownloadPercentile != 40 {
		t.Errorf("groupe opérateur %d mesures / %v %% , attendu 15 / 40",
			res.OperatorSampleSize, res.OperatorDownloadPercentile)
	}
	if res.OperatorTech != "4G" || res.OperatorTechSampleSize != 12 {
		t.Fatalf("sous-groupe techno %q / %d, attendu 4G / 12", res.OperatorTech, res.OperatorTechSampleSize)
	}
	// Parmi Orange 4G : 6,5 bat 6 mesures sur 12 (1 à 6).
	if res.OperatorTechDownloadPercentile != 50 {
		t.Errorf("centile Orange 4G %v, attendu 50", res.OperatorTechDownloadPercentile)
	}

	// Neuf mesures 4G seulement : le sous-groupe n'est pas renvoyé, mais le
	// groupe opérateur (9 ≥ 5) reste calculé.
	var peu []Document
	for i := 1; i <= 9; i++ {
		d := doc("Orange BF", "Orange", itoa(i), "1", "50")
		d.CellularTech = "4G"
		peu = append(peu, d)
	}
	p := barometerFrom(peu, BarometerParams{Download: 5.5, Operator: "Orange BF", Tech: "4G"})
	// Parmi Orange : dls 1..9 ; 5,5 bat 5 mesures sur 9.
	if p.OperatorSampleSize != 9 || p.OperatorDownloadPercentile != 55.56 {
		t.Errorf("groupe opérateur %d / %v, attendu 9 / 44.44",
			p.OperatorSampleSize, p.OperatorDownloadPercentile)
	}
	if p.OperatorTech != "" || p.OperatorTechSampleSize != 0 {
		t.Errorf("9 mesures 4G : sous-groupe %q / %d, attendu absent",
			p.OperatorTech, p.OperatorTechSampleSize)
	}
}

// IMP-09 v2 : sous-groupe de zone (~10 km), renvoyé à partir de
// minSubgroupSample mesures seulement ; les mesures sans position sont
// exclues, et sans position transmise il n'y a pas de comparaison de zone.
func TestBarometerFromZone(t *testing.T) {
	ouaga := [2]float64{12.3714, -1.5197}
	var docs []Document
	// 11 mesures à Ouagadougou (même point, distance nulle), dl 10 à 20.
	for i := 10; i <= 20; i++ {
		d := doc("Orange BF", "Orange", itoa(i), "1", "50")
		d.Latitude, d.Longitude = ouaga[0], ouaga[1]
		docs = append(docs, d)
	}
	// 3 mesures à Bobo-Dioulasso (~210 km), exclues de la zone.
	for _, dl := range []string{"50", "60", "70"} {
		d := doc("Orange BF", "Orange", dl, "1", "50")
		d.Latitude, d.Longitude = 11.1771, -4.2979
		docs = append(docs, d)
	}
	// 1 mesure sans position, exclue.
	docs = append(docs, doc("Orange BF", "Orange", "80", "1", "50"))

	res := barometerFrom(docs, BarometerParams{
		Download: 13.5, Operator: "Orange BF", Lat: ouaga[0], Lng: ouaga[1],
	})
	if res.ZoneSampleSize != 11 {
		t.Fatalf("zone %d mesures, attendu 11", res.ZoneSampleSize)
	}
	// Parmi les 11 mesures locales : 13,5 bat 4 mesures (10 à 13).
	if res.ZoneDownloadPercentile != 36.36 {
		t.Errorf("centile zone %v, attendu 36.36", res.ZoneDownloadPercentile)
	}

	sansZone := barometerFrom(docs, BarometerParams{Download: 13.5, Operator: "Orange BF"})
	if sansZone.ZoneSampleSize != 0 {
		t.Errorf("sans position : zone %d, attendu 0", sansZone.ZoneSampleSize)
	}
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// Sans opérateur, ou avec trop peu de mesures, pas de comparaison par
// opérateur (mais le centile national reste calculé).
func TestBarometerFromSansComparaisonOperateur(t *testing.T) {
	docs := []Document{
		doc("Telecel Faso", "Telecel", "10", "2", "50"),
		doc("Telecel Faso", "Telecel", "20", "4", "40"),
		doc("Orange Burkina Faso", "Orange", "30", "6", "30"),
	}

	sans := barometerFrom(docs, BarometerParams{Download: 15, Upload: 3, Ping: 45})
	if sans.Operator != "" || sans.OperatorSampleSize != 0 {
		t.Errorf("opérateur %q / %d, attendu aucun", sans.Operator, sans.OperatorSampleSize)
	}
	if sans.DownloadPercentile != 33.33 {
		t.Errorf("centile national %v, attendu 33.33", sans.DownloadPercentile)
	}

	peu := barometerFrom(docs, BarometerParams{Download: 15, Upload: 3, Ping: 45, Operator: "Telecel"})
	if peu.OperatorSampleSize != 2 || peu.OperatorDownloadPercentile != 0 || peu.OperatorAvgDownload != 0 {
		t.Errorf("2 mesures Telecel : %+v, attendu échantillon 2 sans centile", peu)
	}
}

// IMP-09 v2 : écho du segment de comparaison — « wifi »/« mobile » quand
// l'app en transmet un, vide sinon (un serveur ancien qui n'applique pas
// le filtrage ne doit pas faire croire à une comparaison segmentée).
func TestBarometerEchoSegment(t *testing.T) {
	docs := []Document{doc("Orange BF", "Orange", "10", "2", "50")}
	wifi := barometerFrom(docs, BarometerParams{Download: 5, NetworkType: " wifi "})
	if wifi.NetworkType != "wifi" {
		t.Errorf("écho %q, attendu wifi", wifi.NetworkType)
	}
	mobile := barometerFrom(docs, BarometerParams{Download: 5, NetworkType: "MOBILE"})
	if mobile.NetworkType != "mobile" {
		t.Errorf("écho %q, attendu mobile", mobile.NetworkType)
	}
	sans := barometerFrom(docs, BarometerParams{Download: 5})
	if sans.NetworkType != "" {
		t.Errorf("écho %q, attendu vide", sans.NetworkType)
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
