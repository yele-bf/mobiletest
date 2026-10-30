package mongodb

// validMinMax garde un couple min / max cohérent (ISS-06) : les deux présents,
// positifs ou nuls et min <= max. Sinon le couple est « non mesuré » : un
// min sans max, ou inversé, viendrait d'un client défectueux.
func validMinMax(lo, hi *float64) (*float64, *float64) {
	if lo == nil || hi == nil || *lo < 0 || *hi < *lo {
		return nil, nil
	}
	a, b := round2(*lo), round2(*hi)
	return &a, &b
}
