package vision

import (
	"image"
	"image/color"
	"math"
)

type homography [9]float64

func project(h homography, p Point) Point {
	d := h[6]*p.X + h[7]*p.Y + h[8]
	return Point{(h[0]*p.X + h[1]*p.Y + h[2]) / d, (h[3]*p.X + h[4]*p.Y + h[5]) / d}
}
func fit(src, dst []Point) (homography, bool) {
	var a [8][9]float64
	for i := 0; i < 4; i++ {
		x, y, u, v := src[i].X, src[i].Y, dst[i].X, dst[i].Y
		a[2*i] = [9]float64{x, y, 1, 0, 0, 0, -u * x, -u * y, u}
		a[2*i+1] = [9]float64{0, 0, 0, x, y, 1, -v * x, -v * y, v}
	}
	for c := 0; c < 8; c++ {
		pivot := c
		for r := c + 1; r < 8; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[pivot][c]) {
				pivot = r
			}
		}
		if math.Abs(a[pivot][c]) < 1e-9 {
			return homography{}, false
		}
		a[c], a[pivot] = a[pivot], a[c]
		v := a[c][c]
		for j := c; j < 9; j++ {
			a[c][j] /= v
		}
		for r := 0; r < 8; r++ {
			if r != c {
				v = a[r][c]
				for j := c; j < 9; j++ {
					a[r][j] -= v * a[c][j]
				}
			}
		}
	}
	var h homography
	for i := 0; i < 8; i++ {
		h[i] = a[i][8]
	}
	h[8] = 1
	return h, true
}
func grayImage(img image.Image) *image.Gray {
	b := img.Bounds()
	scale := math.Min(1, 1600/float64(max(b.Dx(), b.Dy())))
	w, h := int(float64(b.Dx())*scale), int(float64(b.Dy())*scale)
	g := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			g.SetGray(x, y, color.GrayModel.Convert(img.At(b.Min.X+int(float64(x)/scale), b.Min.Y+int(float64(y)/scale))).(color.Gray))
		}
	}
	return g
}
func sample(g *image.Gray, p Point) float64 {
	if p.X < 0 || p.Y < 0 || p.X >= float64(g.Rect.Dx()-1) || p.Y >= float64(g.Rect.Dy()-1) {
		return 255
	}
	x, y := int(p.X), int(p.Y)
	fx, fy := p.X-float64(x), p.Y-float64(y)
	a, b, c, d := float64(g.GrayAt(x, y).Y), float64(g.GrayAt(x+1, y).Y), float64(g.GrayAt(x, y+1).Y), float64(g.GrayAt(x+1, y+1).Y)
	return a*(1-fx)*(1-fy) + b*fx*(1-fy) + c*(1-fx)*fy + d*fx*fy
}

// Connected dark components distinguish filled square fiducials from bubble
// rings and QR finder patterns. Only dense approximately-square blobs qualify.
func findMarkers(g *image.Gray) ([]Point, bool) {
	w, h := g.Rect.Dx(), g.Rect.Dy()
	seen := make([]bool, w*h)
	candidates := []Point{}
	queue := make([]int, 0, 4096)
	// Otsu threshold from the actual upload (not client quality assertions).
	var hist [256]int
	for _, v := range g.Pix {
		hist[v]++
	}
	total := w * h
	sum := 0.
	for i, n := range hist {
		sum += float64(i * n)
	}
	count, s0, best, threshold := 0, 0., -1., 100
	for t, n := range hist {
		count += n
		s0 += float64(t * n)
		if count == 0 || count == total {
			continue
		}
		d := s0/float64(count) - (sum-s0)/float64(total-count)
		v := float64(count) * float64(total-count) * d * d
		if v > best {
			best = v
			threshold = t
		}
	}
	threshold = min(180, max(45, threshold+10))
	for start := 0; start < w*h; start++ {
		if seen[start] || int(g.Pix[start]) > threshold {
			continue
		}
		seen[start] = true
		queue = queue[:0]
		queue = append(queue, start)
		minx, maxx, miny, maxy := w, 0, h, 0
		sumx, sumy := 0., 0.
		count := 0
		for len(queue) > 0 {
			p := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			count++
			x, y := p%w, p/w
			minx = min(minx, x)
			maxx = max(maxx, x)
			miny = min(miny, y)
			maxy = max(maxy, y)
			sumx += float64(x)
			sumy += float64(y)
			for _, n := range []int{p - 1, p + 1, p - w, p + w} {
				if n < 0 || n >= w*h || seen[n] || int(g.Pix[n]) > threshold {
					continue
				}
				nx := n % w
				if math.Abs(float64(nx-x)) > 1 {
					continue
				}
				seen[n] = true
				queue = append(queue, n)
				// A giant dark component is not our fixed card. Bound frontier
				// memory rather than allocating a queue proportional to an upload.
				if len(queue) > w*h/50 {
					return nil, false
				}
			}
		}
		cw, ch := maxx-minx+1, maxy-miny+1
		density := float64(count) / float64(cw*ch)
		aspect := float64(cw) / float64(ch)
		if count >= 36 && float64(count) < float64(w*h)*.006 && density >= .64 && aspect > .48 && aspect < 2.1 {
			candidates = append(candidates, Point{sumx / float64(count), sumy / float64(count)})
		}
	}
	if len(candidates) < 4 {
		return nil, false
	}
	// Extremal convex quadrilateral, clockwise. Rotation is resolved by QR and
	// asymmetric black/white references, not a client-provided orientation.
	out := make([]Point, 4)
	scores := [4]float64{math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(1)}
	for _, p := range candidates {
		vals := [4]float64{p.X + p.Y, -p.X + p.Y, -p.X - p.Y, p.X - p.Y}
		for j, s := range vals {
			if s < scores[j] {
				scores[j] = s
				out[j] = p
			}
		}
	}
	area := 0.
	for i, p := range out {
		q := out[(i+1)%4]
		if math.Hypot(p.X-q.X, p.Y-q.Y) < float64(min(w, h))*.28 {
			return nil, false
		}
		area += p.X*q.Y - q.X*p.Y
	}
	if area/2 < float64(w*h)*.22 {
		return nil, false
	}
	return out, true
}
func boxMean(g *image.Gray, h homography, b Box) (float64, float64) {
	sum, sum2, n := 0., 0., 0.
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			p := Point{b.X + b.Width*(.2+.6*float64(x)/11), b.Y + b.Height*(.2+.6*float64(y)/11)}
			v := sample(g, project(h, p))
			sum += v
			sum2 += v * v
			n++
		}
	}
	mean := sum / n
	return mean, math.Sqrt(math.Max(0, sum2/n-mean*mean))
}
func sharpness(g *image.Gray, h homography, m Manifest, contrast float64) float64 {
	sum := 0.
	for _, p := range m.Markers {
		peak := 0.
		prev := sample(g, project(h, Point{p.X - m.MarkerSize, p.Y}))
		for i := 1; i <= 40; i++ {
			v := sample(g, project(h, Point{p.X - m.MarkerSize + float64(i)*m.MarkerSize/20, p.Y}))
			peak = math.Max(peak, math.Abs(v-prev))
			prev = v
		}
		sum += peak / contrast
	}
	return sum / 4
}
