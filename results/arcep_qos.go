// Bloc « qos » du référentiel ARCEP côté API (IMP-06).
//
// Une seule représentation de ces seuils dans le code : `arcepConfig.Qos`
// (results/arcep.go) les décode avec ces types, et `loadArcepFile` les valide
// au démarrage avec `validate()`. Les routes `/api/arcep/*` les renvoient aux
// clients, qui les affichent avec les valeurs du test.
//
// Les valeurs ne sont pas recopiées ici : la source de vérité reste
// `config/seuils.json` (version 3), partagée avec le scoring de l'application.
//
// Règle d'honnêteté (cf. champ `method_note`) : la comparaison ARCEP est
// SIMPLIFIÉE (moyenne vs seuil), pas la méthode officielle (90 % des mesures
// au-dessus du seuil). Aucune réponse ne conclut « conforme ».
package results

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ArcepQosSeuils est le bloc « qos » de `config/seuils.json` (version 3).
type ArcepQosSeuils struct {
	Navigation   ArcepQosNavigation `json:"navigation"`
	Streaming    ArcepQosStreaming  `json:"streaming"`
	QualiteVideo ArcepQosVideo      `json:"qualite_video,omitempty"`
}

// ArcepQosNavigation porte les seuils QoS nationaux de la navigation web.
type ArcepQosNavigation struct {
	// PageLoadS : temps de chargement maximal d'une page, en secondes (30 s).
	PageLoadS float64 `json:"page_load_s"`
	// PageLoadPct : part minimale de pages sous PageLoadS (95 %).
	PageLoadPct float64 `json:"page_load_pct"`
	// ConnectionS : temps maximal d'établissement de connexion (20 s).
	ConnectionS float64 `json:"connection_s"`
	// TransferPct : part minimale de connexions sous ConnectionS (95 %).
	TransferPct float64 `json:"transfer_pct"`
	// Source : référence documentaire du seuil, affichable telle quelle.
	Source string `json:"source,omitempty"`
}

// ArcepQosStreaming porte les seuils QoS nationaux du streaming vidéo.
type ArcepQosStreaming struct {
	// PreloadS : démarrage maximal de la lecture, en secondes (5 s).
	PreloadS float64 `json:"preload_s"`
	// QualityPct : part minimale de sessions lancées sous PreloadS (95 %).
	QualityPct float64 `json:"quality_pct"`
}

// ArcepQosVideo porte la définition Yélé d'une session vidéo de bonne qualité.
// Ces valeurs ne viennent pas d'ARCEP : le rapport ne définit pas la « bonne
// qualité ». Elles ne doivent jamais être présentées comme officielles.
type ArcepQosVideo struct {
	RebufferRatioMax float64 `json:"rebuffer_ratio_max"`
	ResolutionMin    string  `json:"resolution_min"`
	InterruptionsMax int     `json:"interruptions_max"`
}

// ParseArcepQosSeuils décode le bloc « qos » d'un fichier `seuils.json`.
//
// Le fichier est celui de `config/seuils.json` (référentiel versionné du
// dépôt) : les seuils ne sont pas recopiés dans le code, seule leur forme l'est.
func ParseArcepQosSeuils(raw []byte) (*ArcepQosSeuils, error) {
	var document struct {
		Qos *ArcepQosSeuils `json:"qos"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("arcep qos: décodage du référentiel impossible: %w", err)
	}
	if document.Qos == nil {
		return nil, errors.New("arcep qos: bloc « qos » absent du référentiel")
	}
	if err := document.Qos.validate(); err != nil {
		return nil, err
	}
	return document.Qos, nil
}

// validate refuse un référentiel incomplet : un seuil nul produirait un score
// qui validerait n'importe quelle mesure.
func (s *ArcepQosSeuils) validate() error {
	if s == nil {
		return errors.New("arcep qos: référentiel nil")
	}
	if s.Navigation.PageLoadS <= 0 {
		return errors.New("arcep qos: navigation.page_load_s doit être > 0")
	}
	if s.Navigation.PageLoadPct <= 0 || s.Navigation.PageLoadPct > 100 {
		return errors.New("arcep qos: navigation.page_load_pct doit être dans ]0, 100]")
	}
	if s.Navigation.ConnectionS <= 0 {
		return errors.New("arcep qos: navigation.connection_s doit être > 0")
	}
	if s.Streaming.PreloadS <= 0 {
		return errors.New("arcep qos: streaming.preload_s doit être > 0")
	}
	if s.Streaming.QualityPct <= 0 || s.Streaming.QualityPct > 100 {
		return errors.New("arcep qos: streaming.quality_pct doit être dans ]0, 100]")
	}
	return nil
}
