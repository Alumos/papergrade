package vision

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

func TestManifestGeometryAndIsolation(t *testing.T) {
	qs := make([]Question, 40)
	for i := range qs {
		qs[i] = Question{fmt.Sprintf("q%d", i), fmt.Sprint(i + 1), "single_choice", []string{"A", "B", "C", "D", "E"}}
	}
	m, err := BuildManifest(qs)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Bubbles) != 200 || len(m.Markers) != 4 {
		t.Fatal("incorrect capacity")
	}
	qs[0].Options[0] = "mutated"
	if m.Questions[0].Options[0] != "A" {
		t.Fatal("manifest aliases caller options")
	}
	if err = validateManifest(m); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var restored Manifest
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if err = validateManifest(restored); err != nil {
		t.Fatal(err)
	}
	restored.Markers[0].X = .06
	if err = validateManifest(restored); err == nil {
		t.Fatal("accepted modified marker")
	}
	if m.Bubbles[0].X != .17 || m.Bubbles[0].Y != .25 || m.Bubbles[100].X != .60 {
		t.Fatal("layout drift")
	}
	if math.Abs(m.QRBox.Width*210-m.QRBox.Height*297) > 1e-10 {
		t.Fatal("QR not physically square")
	}
}
func TestManifestInvalidText(t *testing.T) {
	base := Question{"q", "1", "single_choice", []string{"A", "B"}}
	for _, value := range []string{"", "\x00", string([]byte{0xff})} {
		q := base
		q.Number = value
		if _, err := BuildManifest([]Question{q}); err == nil {
			t.Fatalf("accepted bad number %q", value)
		}
	}
	for _, count := range []int{0, 41} {
		if _, err := BuildManifest(make([]Question, count)); err == nil {
			t.Fatal("capacity accepted")
		}
	}
}
