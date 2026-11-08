package vision

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"unicode/utf8"

	qrcode "github.com/skip2/go-qrcode"
)

func escaped(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func qrMatrix(token string) ([][]bool, error) {
	if len(token) < 16 || len(token) > 256 || !validText(token) || strings.TrimSpace(token) != token {
		return nil, fmt.Errorf("vision: token must be 16..256 valid UTF-8 bytes")
	}
	q, err := qrcode.New(token, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	return q.Bitmap(), nil
}

// RenderSVG emits a self-contained printable A4 document, with no external
// resources and with all user-supplied text XML-escaped. QR encodes raw token.
func RenderSVG(m Manifest, title, studentLabel, token string) ([]byte, error) {
	if m.Version == StudentVersion {
		return renderStudentSVG(m, title, studentLabel, token)
	}
	if m.Version == EnglishV4Version {
		return renderEnglishV4SVG(m, title, studentLabel, token)
	}
	if m.Version == EnglishVersion {
		return renderEnglishSVG(m, title, studentLabel, token)
	}
	if err := validateManifest(m); err != nil {
		return nil, err
	}
	if !validText(title) || !validText(studentLabel) || len(title) > 500 || len(studentLabel) > 500 {
		return nil, fmt.Errorf("vision: invalid identity text")
	}
	qr, err := qrMatrix(token)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	fmt.Fprint(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="210mm" height="297mm" viewBox="0 0 210 297"><rect width="210" height="297" fill="white"/><g fill="black">`)
	rect := func(b Box) {
		fmt.Fprintf(&out, `<rect x="%.6f" y="%.6f" width="%.6f" height="%.6f"/>`, b.X*210, b.Y*297, b.Width*210, b.Height*297)
	}
	for _, p := range m.Markers {
		rect(Box{p.X - m.MarkerSize/2, p.Y - m.MarkerSize*210/297/2, m.MarkerSize, m.MarkerSize * 210 / 297})
	}
	rect(m.BlackReference)
	fmt.Fprintf(&out, `<rect x="%.6f" y="%.6f" width="%.6f" height="%.6f" fill="white" stroke="black" stroke-width=".2"/>`, m.WhiteReference.X*210, m.WhiteReference.Y*297, m.WhiteReference.Width*210, m.WhiteReference.Height*297)
	n := float64(len(qr))
	for y, row := range qr {
		for x, on := range row {
			if on {
				rect(Box{m.QRBox.X + float64(x)/n*m.QRBox.Width, m.QRBox.Y + float64(y)/n*m.QRBox.Height, m.QRBox.Width / n, m.QRBox.Height / n})
			}
		}
	}
	fmt.Fprint(&out, `<path d="M 26 23 L 30 30 L 22 30 Z"/>`)
	text := func(x, y, size float64, s string) {
		fmt.Fprintf(&out, `<text x="%.3f" y="%.3f" font-size="%.3f" font-family="sans-serif">%s</text>`, x, y, size, escaped(s))
	}
	text(35, 29, 3.2, "TOP / 此端朝上")
	// Constrain long labels without stretching short names. References finish
	// at y=41.505mm; title ascenders must stay below them.
	fittedText := func(x, y, size, width float64, s string) {
		if float64(utf8.RuneCountInString(s))*size > width {
			fmt.Fprintf(&out, `<text x="%.3f" y="%.3f" font-size="%.3f" textLength="%.3f" lengthAdjust="spacingAndGlyphs" font-family="sans-serif">%s</text>`, x, y, size, width, escaped(s))
		} else {
			text(x, y, size, s)
		}
	}
	fittedText(20, 47, 4.4, 125, title)
	fittedText(20, 56, 3.6, 125, studentLabel)
	text(20, 62, 2.8, "填满选项圆圈；勿折叠、污损定位块或二维码。")
	text(20, 66, 2.4, "Fixed A4 · print 100% · a4-v1 · PoC")
	for i, q := range m.Questions {
		col, row := i/20, i%20
		x := .17 + float64(col)*.43
		y := .25 + float64(row)*.031
		fittedText(x*210-14, y*297+1, 3, 10, q.Number)
	}
	for _, b := range m.Bubbles {
		fmt.Fprintf(&out, `<circle cx="%.6f" cy="%.6f" r="%.6f" fill="white" stroke="black" stroke-width=".30"/>`, b.X*210, b.Y*297, b.R*210)
		fittedText(b.X*210-1, b.Y*297-3, 2.3, 10, b.Option)
	}
	text(20, 278, 2.5, "标准个人答题卡 / Keep all four black corner squares visible")
	fmt.Fprint(&out, "</g></svg>")
	return out.Bytes(), nil
}

// RenderPNG is a lightweight geometry/QR raster for diagnostics and synthetic
// tests, NOT a substitute for the SVG print document (identity text is omitted).
// width must be 700..2000, keeping the allocation below MaxPixels.
func RenderPNG(m Manifest, token string, width int) ([]byte, error) {
	if err := validateManifest(m); err != nil {
		return nil, err
	}
	if width < 700 || width > 2000 {
		return nil, ErrImageSize
	}
	var qr [][]bool
	var err error
	if token != "" || m.Version != StudentVersion {
		qr, err = qrMatrix(token)
		if err != nil {
			return nil, err
		}
	}
	im := raster(m, qr, width)
	var b bytes.Buffer
	err = png.Encode(&b, im)
	return b.Bytes(), err
}
func raster(m Manifest, qr [][]bool, width int) *image.Gray {
	height := int(math.Round(float64(width) * m.HeightMM / m.WidthMM))
	im := image.NewGray(image.Rect(0, 0, width, height))
	for i := range im.Pix {
		im.Pix[i] = 255
	}
	rect := func(b Box, c uint8) {
		x0, y0 := int(math.Round(b.X*float64(width))), int(math.Round(b.Y*float64(height)))
		x1, y1 := int(math.Round((b.X+b.Width)*float64(width))), int(math.Round((b.Y+b.Height)*float64(height)))
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				im.SetGray(x, y, color.Gray{c})
			}
		}
	}
	for _, p := range m.Markers {
		rect(Box{p.X - m.MarkerSize/2, p.Y - m.MarkerSize*m.WidthMM/m.HeightMM/2, m.MarkerSize, m.MarkerSize * m.WidthMM / m.HeightMM}, 0)
	}
	rect(m.BlackReference, 0)
	// White reference center intentionally remains unprinted.
	n := float64(len(qr))
	for y, row := range qr {
		for x, on := range row {
			if on {
				rect(Box{m.QRBox.X + float64(x)/n*m.QRBox.Width, m.QRBox.Y + float64(y)/n*m.QRBox.Height, m.QRBox.Width / n, m.QRBox.Height / n}, 0)
			}
		}
	}
	allBubbles := append([]Bubble(nil), m.Bubbles...)
	for _, d := range m.StudentBubbles {
		allBubbles = append(allBubbles, Bubble{X: d.X, Y: d.Y, R: d.R})
	}
	english := m.Version == EnglishVersion || m.Version == EnglishV4Version
	rectangle := m.BubbleW > 0 && m.BubbleH > 0
	halfW, halfH := m.BubbleW/2*float64(width), m.BubbleH/2*float64(height)
	for _, b := range allBubbles {
		cx, cy, r := b.X*float64(width), b.Y*float64(height), b.R*float64(width)
		if rectangle {
			// Wide rectangular fill cell with the letter printed in the centre.
			for y := int(cy - halfH - 1); y <= int(cy+halfH+1); y++ {
				for x := int(cx - halfW - 1); x <= int(cx+halfW+1); x++ {
					dx, dy := math.Abs(float64(x)-cx), math.Abs(float64(y)-cy)
					onEdge := math.Abs(dx-halfW) < .8 && dy <= halfH+1 || math.Abs(dy-halfH) < .8 && dx <= halfW+1
					if onEdge {
						im.SetGray(x, y, color.Gray{0})
					}
				}
			}
			// Match the printed letter size (about 0.44 of the cell height), so this
			// diagnostic image cannot be easier to read than the real card.
			glyphW, glyphH := halfW*.30, halfH*.45
			for y := int(cy - glyphH); y <= int(cy+glyphH); y++ {
				for x := int(cx - glyphW); x <= int(cx+glyphW); x++ {
					im.SetGray(x, y, color.Gray{0})
				}
			}
			continue
		}
		if english {
			// English cards print a square fill cell with the option letter inside.
			// The raster mirrors that: an outlined square plus a small centre mark
			// standing for the printed glyph, so tests exercise real template ink
			// instead of a clean circle the printed card never has.
			for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
				for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
					dx, dy := math.Abs(float64(x)-cx), math.Abs(float64(y)-cy)
					onEdge := math.Abs(dx-r) < .8 && dy <= r+1 || math.Abs(dy-r) < .8 && dx <= r+1
					if onEdge {
						im.SetGray(x, y, color.Gray{0})
					}
				}
			}
			glyph := r * .42
			for y := int(cy - glyph); y <= int(cy+glyph); y++ {
				for x := int(cx - glyph*.7); x <= int(cx+glyph*.7); x++ {
					im.SetGray(x, y, color.Gray{0})
				}
			}
			continue
		}
		for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
			for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
				d := math.Hypot(float64(x)-cx, float64(y)-cy)
				if math.Abs(d-r) < .8 {
					im.SetGray(x, y, color.Gray{0})
				}
			}
		}
	}
	return im
}
