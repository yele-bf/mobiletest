package results

import (
	"testing"
)

// Extrait de `config/seuils.json` version 3 : seuls les champs lus par
// ParseArcepQosSeuils sont conservés, les valeurs sont celles du référentiel.
const arcepQosV3 = `{
  "version": 3,
  "qos": {
    "navigation": {
      "page_load_s": 30,
      "page_load_pct": 95,
      "connection_s": 20,
      "transfer_pct": 95,
      "source": "ARCEP Burkina Faso - presentation QoS du 29/05/2026"
    },
    "streaming": {
      "preload_s": 5,
      "quality_pct": 95
    },
    "qualite_video": {
      "rebuffer_ratio_max": 0.1,
      "resolution_min": "480p",
      "interruptions_max": 0
    }
  }
}`

func TestParseArcepQosSeuils(t *testing.T) {
	seuils, err := ParseArcepQosSeuils([]byte(arcepQosV3))
	if err != nil {
		t.Fatalf("référentiel v3 refusé : %v", err)
	}

	cas := []struct {
		nom     string
		obtenu  float64
		attendu float64
	}{
		{"navigation.page_load_s", seuils.Navigation.PageLoadS, 30},
		{"navigation.page_load_pct", seuils.Navigation.PageLoadPct, 95},
		{"navigation.connection_s", seuils.Navigation.ConnectionS, 20},
		{"navigation.transfer_pct", seuils.Navigation.TransferPct, 95},
		{"streaming.preload_s", seuils.Streaming.PreloadS, 5},
		{"streaming.quality_pct", seuils.Streaming.QualityPct, 95},
		{"qualite_video.rebuffer_ratio_max", seuils.QualiteVideo.RebufferRatioMax, 0.1},
	}
	for _, c := range cas {
		if c.obtenu != c.attendu {
			t.Errorf("%s = %v, attendu %v", c.nom, c.obtenu, c.attendu)
		}
	}

	if seuils.QualiteVideo.ResolutionMin != "480p" {
		t.Errorf("resolution_min = %q, attendu 480p", seuils.QualiteVideo.ResolutionMin)
	}
	if seuils.QualiteVideo.InterruptionsMax != 0 {
		t.Errorf("interruptions_max = %d, attendu 0", seuils.QualiteVideo.InterruptionsMax)
	}
}

func TestParseArcepQosSeuilsRefuseSansBlocQos(t *testing.T) {
	if _, err := ParseArcepQosSeuils([]byte(`{"version": 3, "profiles": {}}`)); err == nil {
		t.Fatal("un référentiel sans bloc qos doit être refusé")
	}
}

// Un seuil à zéro validerait n'importe quelle mesure : il doit être refusé.
func TestParseArcepQosSeuilsRefuseSeuilNul(t *testing.T) {
	brut := `{"version": 3, "qos": {
		"navigation": {"page_load_s": 0, "page_load_pct": 95, "connection_s": 20, "transfer_pct": 95},
		"streaming": {"preload_s": 5, "quality_pct": 95}
	}}`
	if _, err := ParseArcepQosSeuils([]byte(brut)); err == nil {
		t.Fatal("page_load_s = 0 doit être refusé")
	}
}

// La borne haute reste valide : 100 % est un seuil acceptable.
func TestParseArcepQosSeuilsAccepteCentPourCent(t *testing.T) {
	brut := `{"version": 3, "qos": {
		"navigation": {"page_load_s": 30, "page_load_pct": 100, "connection_s": 20, "transfer_pct": 100},
		"streaming": {"preload_s": 5, "quality_pct": 100}
	}}`
	if _, err := ParseArcepQosSeuils([]byte(brut)); err != nil {
		t.Fatalf("un pourcentage de 100 doit être accepté : %v", err)
	}
}
