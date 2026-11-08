package vision

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// renderEnglishV4SVG prints one a5-v4 card: one question per horizontal row,
// wide rectangular fill cells with the option letter inside, and a compact
// legend naming the paper's real sections.
func renderEnglishV4SVG(m Manifest, title, label, token string) ([]byte, error) {
	if err := validateManifest(m); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	fmt.Fprint(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="210mm" height="148.5mm" viewBox="0 0 210 148.5"><rect width="210" height="148.5" fill="white"/>`)
	if err := englishV4Group(&out, m, title, label, token, 0); err != nil {
		return nil, err
	}
	out.WriteString("</svg>")
	return out.Bytes(), nil
}

func englishV4Group(out *bytes.Buffer, m Manifest, title, label, token string, off float64) error {
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
	plan, err := englishV4Grid(m.Questions)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, `<g transform="translate(0 %.3f)" fill="black" font-family="sans-serif">`, off)
	rect := func(x, y, w, h float64, attrs string) {
		fmt.Fprintf(out, `<rect x="%.4f" y="%.4f" width="%.4f" height="%.4f" %s/>`, x, y, w, h, attrs)
	}
	text := func(x, y, size, width float64, s string) {
		fmt.Fprintf(out, `<text x="%.3f" y="%.3f" font-size="%.3f"`, x, y, size)
		if float64(utf8.RuneCountInString(s))*size > width {
			fmt.Fprintf(out, ` textLength="%.3f" lengthAdjust="spacingAndGlyphs"`, width)
		}
		fmt.Fprintf(out, ">%s</text>", escaped(s))
	}
	centred := func(x, y, size, width float64, s string) {
		fmt.Fprintf(out, `<text x="%.3f" y="%.3f" font-size="%.3f" text-anchor="middle"`, x, y, size)
		if float64(utf8.RuneCountInString(s))*size > width {
			fmt.Fprintf(out, ` textLength="%.3f" lengthAdjust="spacingAndGlyphs"`, width)
		}
		fmt.Fprintf(out, ">%s</text>", escaped(s))
	}
	// An empty white rectangle with its label printed inside. Recognition samples
	// the outer ring, so this printed letter never looks like a pencil mark.
	cell := func(cx, cy float64, s string) {
		rect(cx-englishV4CellW/2, cy-englishV4CellH/2, englishV4CellW, englishV4CellH,
			`rx=".25" fill="white" stroke="black" stroke-width=".22"`)
		if s != "" {
			centred(cx, cy+englishV4CellH*0.20, englishV4CellH*0.44, englishV4CellW*0.72, s)
		}
	}
	for _, p := range m.Markers {
		rect(p.X*210-2, p.Y*148.5-2, 4, 4, "")
	}
	text(12, 18, 3.4, 145, title)
	text(12, 23.5, 2.6, 130, label)
	text(12, 28, 1.9, 130, "学号：固定4位，例如19班32号填1932；每行只涂一个数字")

	positions := [4]string{"①", "②", "③", "④"}
	for column := 0; column < 4; column++ {
		y := 32 + 4.4*float64(column)
		centred(16.5, y+0.9, 2.0, 4, positions[column])
		for digit := 0; digit < 10; digit++ {
			cell(22+5.4*float64(digit), y, fmt.Sprint(digit))
		}
	}
	text(88, 32, 2.2, 62, "请用2B铅笔把所选数字/字母涂满涂黑。")
	text(88, 36.2, 2.2, 62, "修改请彻底擦净；请勿折叠或污损定位块。")
	text(88, 40.4, 2.2, 62, "裁开中线后逐张扫描，勿拍整张双联A4。")
	text(88, 44.6, 2.2, 62, "按100%打印；二维码只匹配试卷。")

	rect(88, 48, 4.5, 4.5, "")
	rect(95, 48, 4.5, 4.5, `fill="white" stroke="black" stroke-width=".2"`)
	text(103, 51.6, 1.5, 30, "黑白参考/勿涂")

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

	text(12, 57.6, 1.8, 188, englishV4Legend(m.Questions))

	for i, q := range m.Questions {
		col, row := i/plan.Rows, i%plan.Rows
		x, y := plan.colX(col), plan.rowY(row)
		text(x, y+0.6, 1.7, englishV4NumberW-1, q.Number)
		for j, o := range q.Options {
			cell(x+englishV4NumberW+englishV4Pitch*(float64(j)+0.5), y, o)
		}
	}
	fmt.Fprint(out, `<path d="M 8 130 H 202" stroke="#888" stroke-width=".15" stroke-dasharray="2 2"/><text x="12" y="135.5" font-size="1.8">a5-v4 · 210×148.5mm · 裁开单卡扫描 · 二维码仅匹配试卷</text></g>`)
	return nil
}

// englishV4Legend names every printed group with its real question range, so a
// teacher can find a section even though sections flow across columns.
func englishV4Legend(questions []Question) string {
	if len(questions) == 0 {
		return ""
	}
	parts := []string{}
	start := 0
	for i := 1; i <= len(questions); i++ {
		if i < len(questions) && questions[i].Section == questions[start].Section {
			continue
		}
		parts = append(parts, englishGroupLabel(questions[start].Section, start+1, i))
		start = i
	}
	return strings.Join(parts, "  ·  ")
}
