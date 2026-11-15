package mongodb

import "testing"

// Repli historique (15/11) : signal_dbm devient RSSI en WiFi et RSRP en
// cellulaire, seulement quand le bloc signal détaillé est absent.
func TestConsolidateSignalFallback(t *testing.T) {
	dbm := -58

	// WiFi sans wifi_rssi : RSSI = signal_dbm.
	w := &Document{NetworkType: "WiFi", SignalDbm: dbm}
	consolidateSignal(w)
	if w.Rssi == nil || *w.Rssi != dbm {
		t.Fatalf("WiFi: Rssi attendu %d, obtenu %v", dbm, w.Rssi)
	}
	if w.Rsrp != nil {
		t.Fatalf("WiFi: Rsrp doit rester nil, obtenu %v", *w.Rsrp)
	}

	// Cellulaire sans signal_rsrp : RSRP = signal_dbm, RSSI nil.
	c := &Document{NetworkType: "4G", SignalDbm: dbm}
	consolidateSignal(c)
	if c.Rsrp == nil || *c.Rsrp != dbm {
		t.Fatalf("4G: Rsrp attendu %d, obtenu %v", dbm, c.Rsrp)
	}
	if c.Rssi != nil {
		t.Fatalf("4G: Rssi doit rester nil, obtenu %v", *c.Rssi)
	}

	// Mesures détaillées présentes : elles gagnent, pas d'écrasement.
	d := &Document{NetworkType: "WiFi", SignalDbm: dbm,
		WifiRssi: ptrInt(-45), SignalRsrp: ptrInt(-100)}
	consolidateSignal(d)
	if *d.Rssi != -45 || *d.Rsrp != -100 {
		t.Fatalf("priorité détaillée perdue : Rssi=%v Rsrp=%v", *d.Rssi, *d.Rsrp)
	}

	// signal_dbm à 0 = non mesuré : aucun signal fictif.
	z := &Document{NetworkType: "WiFi", SignalDbm: 0}
	consolidateSignal(z)
	if z.Rssi != nil {
		t.Fatalf("dbm 0: Rssi doit rester nil, obtenu %v", *z.Rssi)
	}

	// Réseau vide : pas de repli du tout.
	e := &Document{NetworkType: "", SignalDbm: dbm}
	consolidateSignal(e)
	if e.Rssi != nil || e.Rsrp != nil {
		t.Fatalf("réseau vide: repli indû (Rssi=%v Rsrp=%v)",
			e.Rssi, e.Rsrp)
	}

	// 5G SA : ancre 5G d'abord, repli signal_dbm sinon.
	nr := &Document{NetworkType: "5G", SignalNrRsrp: ptrInt(-90), SignalDbm: dbm}
	consolidateSignal(nr)
	if *nr.Rsrp != -90 {
		t.Fatalf("5G: SS-RSRP attendu -90, obtenu %v", *nr.Rsrp)
	}
	nr2 := &Document{NetworkType: "5G", SignalDbm: dbm}
	consolidateSignal(nr2)
	if nr2.Rsrp == nil || *nr2.Rsrp != dbm {
		t.Fatalf("5G: repli signal_dbm attendu %d, obtenu %v", dbm, nr2.Rsrp)
	}
}

func ptrInt(v int) *int { return &v }
