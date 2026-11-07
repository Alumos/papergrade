// Package vision implements only the versioned, fixed A4 answer card.
// It is a synthetic-tested PoC, not a validated arbitrary-document or phone OCR system.
package vision

import "errors"

const (
	Version      = "a4-v1"
	MaxQuestions = 40
	MaxPixels    = 8_000_000
)

var (
	ErrInvalidManifest = errors.New("vision: invalid or unsupported manifest")
	ErrImageSize       = errors.New("vision: image must be at least 500px per side and no more than 8MP")
	ErrQuality         = errors.New("vision: image quality rejected")
	ErrMarkers         = errors.New("vision: four reliable corner markers not found")
	ErrQR              = errors.New("vision: personal QR token could not be decoded")
)

type Question struct {
	ID      string   `json:"id"`
	Number  string   `json:"number"`
	Type    string   `json:"type"`
	Options []string `json:"options"`
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Box struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type Bubble struct {
	QuestionID string  `json:"question_id"`
	Option     string  `json:"option"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	R          float64 `json:"r"`
}

// Geometry is normalized to page width and height independently. R and
// MarkerSize are fractions of page WIDTH, so a physical circle remains round.
// Markers are centers in clockwise order: TL, TR, BR, BL.
type Manifest struct {
	Version         string     `json:"version"`
	CoordinateSpace string     `json:"coordinate_space"`
	WidthMM         float64    `json:"width_mm"`
	HeightMM        float64    `json:"height_mm"`
	MarkerSize      float64    `json:"marker_size"`
	Markers         []Point    `json:"markers"`
	QRBox           Box        `json:"qr_box"`
	BlackReference  Box        `json:"black_reference"`
	WhiteReference  Box        `json:"white_reference"`
	Questions       []Question `json:"questions"`
	Bubbles         []Bubble   `json:"bubbles"`
}

type Answer struct {
	QuestionID string   `json:"question_id"`
	Selected   []string `json:"selected"`
	State      string   `json:"state"`
	Confidence float64  `json:"confidence"`
}
type Quality struct {
	Accepted  bool     `json:"accepted"`
	Contrast  float64  `json:"contrast"`
	Sharpness float64  `json:"sharpness"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	Markers   []Point  `json:"markers"`
	Reasons   []string `json:"reasons"`
}
type Recognition struct {
	Token   string   `json:"token"`
	Answers []Answer `json:"answers"`
	Quality Quality  `json:"quality"`
}
