package vision

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"testing"
)

func englishSample(t *testing.T) []EnglishQuestion {
	t.Helper()
	qs := make([]EnglishQuestion, 0, 60)
	add := func(n int, options []string, section string) {
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("q%d", len(qs)+1)
			qs = append(qs, EnglishQuestion{Question: Question{ID: id, Number: fmt.Sprint(len(qs) + 1), Type: "single_choice", Options: options, Section: section}})
		}
	}
	add(20, []string{"A", "B", "C"}, "第一部分：听力")
	add(15, []string{"A", "B", "C", "D"}, "第二部分：阅读理解")
	add(5, []string{"A", "B", "C", "D", "E", "F", "G"}, "第三部分：七选五")
	add(20, []string{"A", "B", "C", "D"}, "第四部分：英语知识运用")
	return qs
}

func TestEnglishManifestFixedGeometry(t *testing.T) {
	m, err := BuildEnglishManifest(englishSample(t))
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != EnglishVersion || m.StudentNumberDigits != 4 {
		t.Fatalf("version/digits %q/%d", m.Version, m.StudentNumberDigits)
	}
	if m.WidthMM != 210 || m.HeightMM != 148.5 {
		t.Fatalf("size %.1f x %.1f", m.WidthMM, m.HeightMM)
	}
	if len(m.Questions) != 60 || len(m.StudentBubbles) != 40 {
		t.Fatalf("questions %d digits %d", len(m.Questions), len(m.StudentBubbles))
	}
	if got := len(m.Bubbles); got != 20*3+15*4+5*7+20*4 {
		t.Fatalf("bubbles %d", got)
	}
	for _, b := range m.Bubbles {
		if b.X < .02 || b.X > .98 || b.Y < .1 || b.Y > .92 {
			t.Fatalf("bubble outside printable area: %+v", b)
		}
	}
	// Re-validation must reject a tampered sampling position.
	tampered := m
	tampered.Bubbles = append([]Bubble(nil), m.Bubbles...)
	tampered.Bubbles[0].X += .01
	if err := validateManifest(tampered); err == nil {
		t.Fatal("moved bubble accepted")
	}
}

func TestEnglishRejectsUnprintable(t *testing.T) {
	if _, err := BuildEnglishManifest(nil); err == nil {
		t.Fatal("empty accepted")
	}
	seven := EnglishQuestion{Question: Question{ID: "x", Number: "1", Type: "single_choice", Options: []string{"A", "B", "C", "D", "E", "F", "G", "H"}}}
	if _, err := BuildEnglishManifest([]EnglishQuestion{seven}); err == nil {
		t.Fatal("eight options accepted")
	}
	one := EnglishQuestion{Question: Question{ID: "x", Number: "1", Type: "single_choice", Options: []string{"A"}}}
	if _, err := BuildEnglishManifest([]EnglishQuestion{one}); err == nil {
		t.Fatal("single option accepted")
	}
	// 60 questions where every band is seven options cannot keep a printable
	// row pitch on half A4 and must be rejected rather than shrunk.
	all7 := make([]EnglishQuestion, 0, 60)
	for i := 0; i < 60; i++ {
		all7 = append(all7, EnglishQuestion{Question: Question{ID: fmt.Sprintf("q%d", i), Number: fmt.Sprint(i + 1), Type: "single_choice", Options: []string{"A", "B", "C", "D", "E", "F", "G"}}})
	}
	if _, err := BuildEnglishManifest(all7); err == nil {
		t.Fatal("over-capacity card accepted")
	}
}

func TestEnglishTwoUpAndManifestRoundTrip(t *testing.T) {
	m, err := BuildEnglishManifest(englishSample(t))
	if err != nil {
		t.Fatal(err)
	}
	svg, err := RenderTwoUpSVG(m, "英语期中考试答题卡", []PrintCard{{StudentLabel: "1932 张三", Token: "exam-release-token"}, {StudentLabel: "1933 李四", Token: "exam-release-token"}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(svg, []byte("height=\"297mm\"")) || !bytes.Contains(svg, []byte("a5-v3")) {
		t.Fatal("two-up svg missing A4 shell or version label")
	}
	if bytes.Count(svg, []byte("<svg")) != 1 {
		t.Fatal("two-up must be one A4 document")
	}
	// A single English card must also render on its own half-page.
	if _, err := renderEnglishSVG(m, "英语", "1932 张三", "exam-release-token"); err != nil {
		t.Fatal(err)
	}
	// The manifest must survive JSON round-trip exactly, including the reduced QR.
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var back Manifest
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if err := validateManifest(back); err != nil {
		t.Fatalf("round-trip manifest rejected: %v", err)
	}
	if back.QRBox.Width*210 > 24.001 {
		t.Fatalf("qr not reduced: %.2fmm", back.QRBox.Width*210)
	}
}

func TestEnglishRenderPNGIsDecodable(t *testing.T) {
	m, err := BuildEnglishManifest(englishSample(t))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := RenderPNG(m, "exam-release-token", 1800)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() < 500 {
		t.Fatalf("render too small: %v", img.Bounds())
	}
}
