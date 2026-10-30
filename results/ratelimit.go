package results

import (
	"sync"
	"time"
)

// rateLimiter compte les requêtes par clé (adresse IP) sur une fenêtre fixe.
// IMP-03 : freine les essais de mots de passe en série sur /api/auth/*.
//
// Au Burkina Faso, les opérateurs mobiles partagent une même IP publique
// entre de nombreux abonnés (CGNAT) : les limites doivent rester larges.
type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]*bucket
	lastGC  time.Time
}

type bucket struct {
	start time.Time
	count int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		buckets: make(map[string]*bucket),
	}
}

// allow compte un envoi pour key et indique s'il reste sous la limite de la
// fenêtre en cours (fenêtre fixe, remise à zéro après `window`).
func (l *rateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Purge périodique des fenêtres expirées, pour que la table ne grossisse
	// pas indéfiniment avec les IP de passage.
	if now.Sub(l.lastGC) > l.window {
		for k, b := range l.buckets {
			if now.Sub(b.start) >= l.window {
				delete(l.buckets, k)
			}
		}
		l.lastGC = now
	}

	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= l.window {
		l.buckets[key] = &bucket{start: now, count: 1}
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}
