package results

import (
	"os"
	"testing"
)
// TestLoadArcepFileReferentiel charge le référentiel réellement livré
// (config/seuils.json) et vérifie les valeurs sur lesquelles l'application
// s'appuie. Il ne réinvente aucun seuil : il constate ceux du fichier, pour
// qu'une édition accidentelle du JSON casse la CI au lieu de fausser les
// scores en silence.
func TestLoadArcepFileReferentiel(t *testing.T) {
	cfg, err := loadArcepFile("../config/seuils.json")
	if err != nil {
		t.Fatalf("référentiel illisible: %v", err)
	}
	if cfg.Version != 3 {
		t.Fatalf("version = %d, attendu 3", cfg.Version)
	}
	if cfg.Source == "" || cfg.MethodNote == "" {
		t.Fatal("source ou method_note vide : la provenance des seuils est obligatoire")
	}
	if len(cfg.Zones) != 10 {
		t.Fatalf("%d zones, attendu 10", len(cfg.Zones))
	}
	want := map[string][2]float64{
		"D1": {0.05, 0.03},
		"D2": {1, 0.512},
		"D3": {2, 1},
		"D4": {4, 2},
	}
	for name, w := range want {
		p, ok := cfg.Profiles[name]
		if !ok {
			t.Fatalf("profil %s absent", name)
		}
		if p.DownloadMbps != w[0] || p.UploadMbps != w[1] {
			t.Fatalf("profil %s = %v/%v, attendu %v/%v", name,
				p.DownloadMbps, p.UploadMbps, w[0], w[1])
		}
	}
	if cfg.RadiusKm != 40 {
		t.Fatalf("radius_km = %v, attendu 40", cfg.RadiusKm)
	}
	if cfg.Qos == nil {
		t.Fatal("bloc qos absent")
	}
	if cfg.Qos.Navigation.PageLoadS != 30 || cfg.Qos.Navigation.ConnectionS != 20 {
		t.Fatalf("seuils navigation = %v/%v, attendu 30/20",
			cfg.Qos.Navigation.PageLoadS, cfg.Qos.Navigation.ConnectionS)
	}
	if cfg.Qos.Streaming.PreloadS != 5 {
		t.Fatalf("seuil streaming = %v, attendu 5", cfg.Qos.Streaming.PreloadS)
	}
	if got := buildAliases(cfg)["bobo"]; got != "Bobo-Dioulasso" {
		t.Fatalf("alias bobo -> %q", got)
	}
}

// TestLoadArcepFileRefuseProfilInconnu : une zone qui pointe un profil absent
// doit être refusée, sinon la comparaison renverrait un seuil zéro.
func TestLoadArcepFileRefuseProfilInconnu(t *testing.T) {
	dir := t.TempDir()
	bad := dir + "/seuils.json"
	if err := os.WriteFile(bad, []byte(`{"version":3,"profiles":{"D4":{"download_mbps":4,"upload_mbps":2}},"zones":{"X":{"profile":"D9"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadArcepFile(bad); err == nil {
		t.Fatal("référentiel avec profil inconnu accepté")
	}
}

// TestLoadArcepFileRefuseQosNul : un seuil QoS à zéro validerait n'importe
// quelle mesure ; le chargement doit échouer.
func TestLoadArcepFileRefuseQosNul(t *testing.T) {
	dir := t.TempDir()
	bad := dir + "/seuils.json"
	if err := os.WriteFile(bad, []byte(`{"version":3,"profiles":{"D4":{"download_mbps":4,"upload_mbps":2}},"zones":{"X":{"profile":"D4"}},"qos":{"navigation":{"page_load_s":0,"page_load_pct":95,"connection_s":20,"transfer_pct":95}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadArcepFile(bad); err == nil {
		t.Fatal("référentiel avec page_load_s = 0 accepté")
	}
}
