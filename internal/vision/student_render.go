package vision

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

func renderStudentSVG(m Manifest, title, label, token string) ([]byte, error) {
	if err := validateManifest(m); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	fmt.Fprint(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="210mm" height="148.5mm" viewBox="0 0 210 148.5"><rect width="210" height="148.5" fill="white"/>`)
	if err := studentGroup(&out, m, title, label, token, 0); err != nil {
		return nil, err
	}
	out.WriteString("</svg>")
	return out.Bytes(), nil
}

// RenderTwoUpSVG places one or two independent v2 cards at unchanged physical
// size on A4. The unused lower half of an odd final page stays blank. Cut first;
// Recognize accepts a single card, never the complete two-card sheet.
func RenderTwoUpSVG(m Manifest, title string, cards []PrintCard) ([]byte, error) {
	if err := validateManifest(m); err != nil {
		return nil, err
	}
	if (m.Version != StudentVersion && m.Version != EnglishVersion && m.Version != EnglishV4Version) || len(cards) < 1 || len(cards) > 2 {
		return nil, fmt.Errorf("vision: two-up requires a5-v2/a5-v3 and 1..2 cards")
	}
	var out bytes.Buffer
	fmt.Fprint(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="210mm" height="297mm" viewBox="0 0 210 297"><rect width="210" height="297" fill="white"/>`)
	for i, c := range cards {
		var err error
		if m.Version == EnglishV4Version {
			err = englishV4Group(&out, m, title, c.StudentLabel, c.Token, float64(i)*148.5)
		} else if m.Version == EnglishVersion {
			err = englishGroup(&out, m, title, c.StudentLabel, c.Token, float64(i)*148.5)
		} else {
			err = studentGroup(&out, m, title, c.StudentLabel, c.Token, float64(i)*148.5)
		}
		if err != nil {
			return nil, err
		}
	}
	fmt.Fprint(&out, `<path d="M 0 148.5 H 210" fill="none" stroke="#888" stroke-width=".15" stroke-dasharray="2 2"/></svg>`)
	return out.Bytes(), nil
}

func studentGroup(out *bytes.Buffer, m Manifest, title, label, token string, offset float64) error {
	if !validText(title) || !validText(label) || len(title) > 500 || len(label) > 500 {
		return fmt.Errorf("vision: invalid identity text")
	}
	var qr [][]bool
	if token != "" {
		var err error
		qr, err = qrMatrix(token)
		if err != nil {
			return err
		}
	}
	fmt.Fprintf(out, `<g transform="translate(0 %.3f)" fill="black" font-family="sans-serif">`, offset)
	rect := func(x, y, w, h float64, attrs string) {
		fmt.Fprintf(out, `<rect x="%.6f" y="%.6f" width="%.6f" height="%.6f" %s/>`, x, y, w, h, attrs)
	}
	text := func(x, y, size, width float64, s string) {
		fmt.Fprintf(out, `<text x="%.3f" y="%.3f" font-size="%.3f"`, x, y, size)
		if float64(utf8.RuneCountInString(s))*size > width {
			fmt.Fprintf(out, ` textLength="%.3f" lengthAdjust="spacingAndGlyphs"`, width)
		}
		fmt.Fprintf(out, ">%s</text>", escaped(s))
	}
	for _, p := range m.Markers {
		rect(p.X*210-2, p.Y*148.5-2, 4, 4, "")
	}
	fmt.Fprint(out, `<path d="M 170 8 L 172 12 L 168 12 Z"/>`)
	text(174, 12, 2.2, 20, "TOP / 朝上")
	text(12, 17, 4.0, 181, title)
	text(12, 24, 3.0, 138, label)
	rect(119, 30, 5, 5, "")
	rect(128, 30, 5, 5, `fill="white" stroke="black" stroke-width=".2"`)
	text(116, 40, 2, 30, "黑白参考 / 勿涂")
	text(12, 28, 2.3, 100, "学号：先手写，再每列填满一个数字（保留前导0）")
	for c := 0; c < m.StudentNumberDigits; c++ {
		rect(20.1+float64(c)*5, 29, 3.8, 3.2, `fill="white" stroke="black" stroke-width=".2"`)
	}
	for d := 0; d < 10; d++ {
		text(15, 35+4.5*float64(d)+.7, 2.2, 4, fmt.Sprint(d))
	}
	circle := func(x, y, r float64) {
		fmt.Fprintf(out, `<circle cx="%.6f" cy="%.6f" r="%.6f" fill="white" stroke="black" stroke-width=".30"/>`, x, y, r)
	}
	for _, d := range m.StudentBubbles {
		circle(d.X*210, d.Y*148.5, d.R*210)
	}
	text(87, 49, 2.5, 62, "请用2B铅笔将圆点涂满、涂黑。")
	text(87, 55, 2.5, 62, "修改请擦净；勿折叠或污损定位块。")
	text(87, 61, 2.5, 62, "裁开后逐张扫描，勿拍整张双联A4。")
	text(87, 67, 2.3, 105, "按100%打印；未完成真实手机/2B纸卡验收。")
	text(87, 73, 2.2, 105, "a5-v2 · 210 × 148.5mm · PoC")
	n := float64(len(qr))
	for y, row := range qr {
		for x, on := range row {
			if on {
				rect((m.QRBox.X+float64(x)/n*m.QRBox.Width)*210, (m.QRBox.Y+float64(y)/n*m.QRBox.Height)*148.5, m.QRBox.Width/n*210, m.QRBox.Height/n*148.5, "")
			}
		}
	}
	text(158, 61, 2, 35, "考试码（非身份码），学号须填涂")
	for i, q := range m.Questions {
		column, row := i/10, i%10
		text(14+48*float64(column), 88+5*float64(row)+.7, 2.4, 10, q.Number)
		// Per-row option labels sit above circles, outside sampled interiors.
		// This remains correct when single/true-false rows have different labels.
		for j, o := range q.Options {
			text(27+48*float64(column)+6*float64(j)-.8, 85.8+5*float64(row), 1.25, 4, o)
		}
	}
	for _, b := range m.Bubbles {
		circle(b.X*210, b.Y*148.5, b.R*210)
	}

	text(15, 143, 2.2, 179, "学号每列只能涂一个数字；单选/判断只能涂一个选项。保持四角黑块完整。")
	out.WriteString("</g>")
	return nil
}
