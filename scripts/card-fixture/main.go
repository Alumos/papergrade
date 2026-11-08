// Command card-fixture creates deterministic test evidence, not a real student card.
// It intentionally lives outside the production binary and is used only by E2E.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/papergrade/papergrade/internal/vision"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"
)

func main() {
	manifestPath := flag.String("manifest", "", "manifest JSON file")
	number := flag.String("number", "0001", "student number to fill")
	token := flag.String("token", "fixture-release-000000000000", "exam release QR, never student ID")
	answers := flag.String("answers", "B", "one comma-separated option set per question")
	output := flag.String("output", "", "destination PNG")
	flag.Parse()
	if *manifestPath == "" || *output == "" {
		panic("manifest and output required")
	}
	raw, err := os.ReadFile(*manifestPath)
	must(err)
	var m vision.Manifest
	must(json.Unmarshal(raw, &m))
	data, err := vision.RenderPNG(m, *token, 1800)
	must(err)
	decoded, err := png.Decode(bytes.NewReader(data))
	must(err)
	img := image.NewRGBA(decoded.Bounds())
	draw.Draw(img, img.Bounds(), decoded, image.Point{}, draw.Src)
	fill := func(x, y, r float64) {
		cx, cy := x*float64(img.Bounds().Dx()), y*float64(img.Bounds().Dy())
		radius := r * float64(img.Bounds().Dx()) * .88
		for yy := int(cy-radius) - 1; yy <= int(cy+radius)+1; yy++ {
			for xx := int(cx-radius) - 1; xx <= int(cx+radius)+1; xx++ {
				dx, dy := float64(xx)-cx, float64(yy)-cy
				if dx*dx+dy*dy <= radius*radius {
					img.Set(xx, yy, color.Black)
				}
			}
		}
	}
	// Decode v2 student_bubbles generically, so this fixture follows the manifest contract.
	var fields map[string]json.RawMessage
	must(json.Unmarshal(raw, &fields))
	var digits []struct {
		Column  int
		Digit   int
		X, Y, R float64
	}
	must(json.Unmarshal(fields["student_bubbles"], &digits))
	if len(*number) != m.StudentNumberDigits {
		panic("number does not match manifest width")
	}
	for _, b := range digits {
		if b.Column >= 0 && b.Column < len(*number) && int((*number)[b.Column]-'0') == b.Digit {
			fill(b.X, b.Y, b.R)
		}
	}
	sets := strings.Split(*answers, ",")
	for i, q := range m.Questions {
		selected := ""
		if i < len(sets) {
			selected = sets[i]
		}
		for _, b := range m.Bubbles {
			if b.QuestionID == q.ID && strings.Contains(selected, b.Option) {
				fill(b.X, b.Y, b.R)
			}
		}
	}
	file, err := os.Create(*output)
	must(err)
	must(png.Encode(file, img))
	must(file.Close())
	fmt.Println("generated synthetic evidence", *output)
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
