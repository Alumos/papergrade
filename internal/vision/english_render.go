package vision

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

func renderEnglishSVG(m Manifest, title, label, token string) ([]byte, error) {
	if err := validateManifest(m); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	fmt.Fprint(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="210mm" height="148.5mm" viewBox="0 0 210 148.5"><rect width="210" height="148.5" fill="white"/>`)
	if err := englishGroup(&out, m, title, label, token, 0); err != nil {
		return nil, err
	}
	out.WriteString("</svg>")
	return out.Bytes(), nil
}

// englishGroup draws one complete card. Nothing is ever printed inside a
// sampling bubble: option letters sit to the left of their bubble and question
// numbers sit above the stack, so OMR never reads its own labels.
func englishGroup(out *bytes.Buffer, m Manifest, title, label, token string, off float64) error {
	if !validText(title) || !validText(label) || len(title) > 500 || len(label) > 500 {
		return fmt.Errorf("vision: invalid identity text")
	}
	var qr [][]bool
	if token != "" {
		matrix, err := qrMatrix(token)
		if err != nil {
			return err
		}
		qr = matrix
	}
	fmt.Fprintf(out, `<g transform="translate(0 %.3f)" fill="black" font-family="sans-serif">`, off)
	rect := func(x, y, w, h float64, a string) {
		fmt.Fprintf(out, `<rect x="%.4f" y="%.4f" width="%.4f" height="%.4f" %s/>`, x, y, w, h, a)
	}
	text := func(x, y, size, width float64, s string) {
		fmt.Fprintf(out, `<text x="%.3f" y="%.3f" font-size="%.3f"`, x, y, size)
		if float64(utf8.RuneCountInString(s))*size > width {
			fmt.Fprintf(out, ` textLength="%.3f" lengthAdjust="spacingAndGlyphs"`, width)
		}
		fmt.Fprintf(out, ">%s</text>", escaped(s))
	}
	// Empty, outlined fill targets. The label letter/number is never drawn inside
	// a fill target, so the OMR sampling region stays uncontaminated.
	textCenter := func(x, y, size, width float64, s string) {
		fmt.Fprintf(out, `<text x="%.3f" y="%.3f" font-size="%.3f" text-anchor="middle"`, x, y, size)
		if float64(utf8.RuneCountInString(s))*size > width {
			fmt.Fprintf(out, ` textLength="%.3f" lengthAdjust="spacingAndGlyphs"`, width)
		}
		fmt.Fprintf(out, ">%s</text>", escaped(s))
	}
	// A fill cell is an empty white square with its option letter printed inside,
	// matching the usual answer-sheet look. Recognition samples the outer ring so
	// this printed letter is never mistaken for a student's pencil mark.
	cell := func(x, y, size float64, label string) {
		fmt.Fprintf(out, `<rect x="%.4f" y="%.4f" width="%.4f" height="%.4f" rx=".25" fill="white" stroke="black" stroke-width=".22"/>`, x-size/2, y-size/2, size, size)
		if label != "" {
			textCenter(x, y+size*0.20, size*0.44, size*0.72, label)
		}
	}
	for _, p := range m.Markers {
		rect(p.X*210-2, p.Y*148.5-2, 4, 4, "")
	}
	text(12, 17, 3.6, 145, title)
	text(12, 22.5, 2.7, 130, label)
	text(12, 27, 2.0, 140, "学号填涂：4位数字，例如19班32号填1932；每行只能涂一个数字")

	// Fixed four-digit identity block: one horizontal row per digit position, with
	// the digit printed inside each empty cell.
	positions := [4]string{"①", "②", "③", "④"}
	for column := 0; column < 4; column++ {
		y := 34 + englishPitchY*float64(column)
		textCenter(20.5, y+0.9, 2.0, 4, positions[column])
		for digit := 0; digit < 10; digit++ {
			cell(24+englishPitchY*float64(digit), y, 3.4, fmt.Sprint(digit))
		}
	}
	text(70, 33, 2.3, 64, "请用2B铅笔把所选数字/字母涂满涂黑。")
	text(70, 37.5, 2.3, 64, "修改请彻底擦净；请勿折叠或污损定位块。")
	text(70, 42, 2.3, 64, "裁开中线后逐张扫描，勿拍整张双联A4。")
	text(70, 46.5, 2.3, 64, "按100%打印；二维码只匹配试卷。")

	rect(140, 31, 5, 5, "")
	rect(149, 31, 5, 5, `fill="white" stroke="black" stroke-width=".2"`)
	text(140, 40.2, 1.5, 24, "黑白参考/勿涂")

	if len(qr) > 0 {
		n := float64(len(qr))
		for y, row := range qr {
			for x, on := range row {
				if on {
					rect((m.QRBox.X+float64(x)/n*m.QRBox.Width)*210, (m.QRBox.Y+float64(y)/n*m.QRBox.Height)*148.5, m.QRBox.Width/n*210, m.QRBox.Height/n*148.5, "")
				}
			}
		}
		text(166, 53.4, 1.5, 26, "试卷码（非学生身份）")
	}

	maxBand := [englishBands]int{2, 2, 2}
	for i, q := range m.Questions {
		if n := len(q.Options); n > maxBand[i/englishCols] {
			maxBand[i/englishCols] = n
		}
	}
	starts := englishStarts(maxBand)
	for band := 0; band < englishBands; band++ {
		first := band * englishCols
		if first >= len(m.Questions) {
			continue
		}
		text(12, starts[band]-5.2, 2.1, 120, englishBandHeading(m, first))
	}
	for i, q := range m.Questions {
		band, col := i/englishCols, i%englishCols
		x := englishColumnX(col)
		textCenter(x, starts[band]-2.7, 1.6, 7, q.Number)
		for j, o := range q.Options {
			cell(x, starts[band]+englishPitchY*float64(j), 3.2, o)
		}
	}
	fmt.Fprint(out, `<path d="M 8 137 H 202" stroke="#888" stroke-width=".15" stroke-dasharray="2 2"/><text x="12" y="143.4" font-size="1.8">a5-v3 · 210×148.5mm · 裁开单卡扫描 · 二维码仅匹配试卷</text></g>`)
	return nil
}

// englishBandHeading names every printed group that starts inside one option
// band. A band may hold the end of one section and the start of the next, and
// each group keeps its own question range instead of being mislabelled.
func englishBandHeading(m Manifest, first int) string {
	last := first + englishCols
	if last > len(m.Questions) {
		last = len(m.Questions)
	}
	parts := []string{}
	runStart := first
	for i := first + 1; i <= last; i++ {
		if i < last && m.Questions[i].Section == m.Questions[runStart].Section {
			continue
		}
		parts = append(parts, englishGroupLabel(m.Questions[runStart].Section, runStart+1, i))
		runStart = i
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts, "  ·  ")
}

func englishGroupLabel(section string, from, to int) string {
	if section == "" {
		return fmt.Sprintf("第 %d–%d 题", from, to)
	}
	// Authors usually write the range themselves ("Listening 1-20"). Only add it
	// when the heading carries no digits, so it is never printed twice.
	for _, r := range section {
		if r >= '0' && r <= '9' || r >= '０' && r <= '９' {
			return section
		}
	}
	return fmt.Sprintf("%s（%d–%d）", section, from, to)
}
