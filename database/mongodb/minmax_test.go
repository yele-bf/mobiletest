package mongodb

import (
	"testing"

	"github.com/librespeed/speedtest-go/database/schema"
)

// ISS-06 : min / max conservés quand ils sont cohérents, « non mesurés »
// sinon (absents, incomplets, inversés, négatifs).
func TestToDocumentMinMax(t *testing.T) {
	m := &MongoDB{}
	doc := m.toDocument(&schema.TelemetryData{Extra: `{"type":"active",
		"downloadMin":3.204,"downloadMax":41,
		"uploadMin":2.3,"uploadMax":9.8,
		"pingMin":38,"pingMax":97}`})
	check := func(name string, got *float64, want float64) {
		if got == nil || *got != want {
			t.Errorf("%s : %v, attendu %v", name, got, want)
		}
	}
	check("downloadMin", doc.DownloadMin, 3.2)
	check("downloadMax", doc.DownloadMax, 41)
	check("uploadMin", doc.UploadMin, 2.3)
	check("uploadMax", doc.UploadMax, 9.8)
	check("pingMin", doc.PingMin, 38)
	check("pingMax", doc.PingMax, 97)

	for _, extra := range []string{
		`{"type":"active"}`,
		`{"type":"active","pingMin":38}`,
		`{"type":"active","pingMin":97,"pingMax":38}`,
		`{"type":"active","pingMin":-1,"pingMax":38}`,
	} {
		d := m.toDocument(&schema.TelemetryData{Extra: extra})
		if d.PingMin != nil || d.PingMax != nil {
			t.Errorf("%s : latence min/max %v/%v, attendu non mesurés", extra, d.PingMin, d.PingMax)
		}
	}
}
