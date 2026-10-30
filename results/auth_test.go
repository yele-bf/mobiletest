package results

import (
	"net/http/httptest"
	"testing"
	"time"
)

// IMP-03 : limite des tentatives de connexion par IP.
func TestRateLimiter(t *testing.T) {
	l := newRateLimiter(3, time.Minute)
	t0 := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if !l.allow("1.2.3.4", t0) {
			t.Fatalf("tentative %d refusée sous la limite", i+1)
		}
	}
	if l.allow("1.2.3.4", t0.Add(time.Second)) {
		t.Error("4e tentative acceptée au-delà de la limite")
	}
	if !l.allow("5.6.7.8", t0.Add(time.Second)) {
		t.Error("une autre IP ne doit pas être bloquée")
	}
	if !l.allow("1.2.3.4", t0.Add(time.Minute)) {
		t.Error("la fenêtre suivante doit repartir de zéro")
	}
}

func TestValidEmail(t *testing.T) {
	for e, want := range map[string]bool{
		"awa@exemple.bf":     true,
		" Awa@Exemple.BF ":   true,
		"sans-arobase.bf":    false,
		"@exemple.bf":        false,
		"awa@sanspoint":      false,
		"awa <x>@exemple.bf": false,
	} {
		if got := validEmail(e); got != want {
			t.Errorf("validEmail(%q) = %v, attendu %v", e, got, want)
		}
	}
}

func TestBearerToken(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/me", nil)
	if bearerToken(r) != "" {
		t.Error("sans en-tête : jeton vide attendu")
	}
	r.Header.Set("Authorization", "Bearer  abc123 ")
	if got := bearerToken(r); got != "abc123" {
		t.Errorf("jeton %q, attendu abc123", got)
	}
	r.Header.Set("Authorization", "Basic abc123")
	if bearerToken(r) != "" {
		t.Error("schéma Basic : jeton vide attendu")
	}
}
