# Fixed A4 vision PoC

V0.2 adds a5-v2 student-number half-A4 cards; see [V02 geometry](V02_GEOMETRY.md). The sections below describe legacy a4-v1 behavior and its original opt-in cloud rollout. V2 deterministic OMR is the new default, bounded by backend single-worker admission; it does not require a card gateway. V2 QR is optional shared release metadata, never student identity.

This package is intentionally scoped to one standard card, not arbitrary old
cards. **Synthetic tests are not real-phone acceptance.** No trained model,
OpenCV, GPU stack, subprocess, PDF renderer, network service, or background
worker is introduced. Local recognition is an experimental opt-in; the backend
must keep it disabled unless LOCAL_OMR_ENABLED=true. Default recognition should
use the configured cloud card provider, or report not configured.

## Stable Go surface

- BuildManifest([]Question) (Manifest, error): one to forty questions, two to
  five unique option labels; true_false exactly two; no correct answers stored.
- RenderSVG(Manifest, title, studentLabel, token string) ([]byte, error):
  printable 210 x 297mm A4, four squares, TOP arrow, identity, black/white
  references and a personal QR. All supplied text is XML escaped; invalid XML
  controls/UTF-8 are rejected. Print at 100%, without page headers/footers.
- Recognize(image.Image, Manifest) (Recognition, error): server detects corners
  from pixels, resolves rotation with reference patches and QR, fits a projective
  homography, decodes QR via gozxing, then samples calibrated bubble interiors.
  No candidate token, client marker positions, or client answers are accepted.
- RenderPNG(Manifest, token string, width int) ([]byte, error): lightweight
  diagnostic geometry raster only; **identity text and guidance are omitted**,
  so use SVG for actual printing. Width 700..2000.

Question has ID, Number, Type (single_choice/multiple_choice/true_false), Options.
Recognition has Token, Answers, Quality. Each Answer has QuestionID, Selected,
State (blank/answered/invalid_multiple/uncertain), Confidence. Selected remains
an empty JSON array for blank. Quality.Accepted means the global geometry/QR
checks passed, not that every answer is certain. Any uncertain answer must go
to human review. Confidence is a heuristic, **not a calibrated probability**.
On error never persist an automatic grade; Quality.Reasons records the stage.
Quality.Markers are coordinates in the internally downscaled image (long side
at most 1600), ordered in canonical card orientation, not upload pixels.

## Geometry JSON / browser guidance

Manifest fields use snake_case. coordinate_space is normalized. x and width are
fractions of page width; y and height are fractions of page height. Bubble r
and marker_size are fractions of page **width**. Thus physical squares/circles
have unequal normalized x/y dimensions. Version is a4-v1.

- markers centers clockwise: (.05,.05), (.95,.05), (.95,.95), (.05,.95)
- marker_size: .024 (5.04mm side, normalized height .024*210/297)
- qr_box: x=.76 y=.075 width=.17 height=.17*210/297; quiet zone included
- black_reference: x=.56 y=.115 width=.035 height=.035*210/297
- white_reference: x=.61 y=.115, same size; sample inner patch, not border
- bubbles: {question_id, option, x, y, r}; r=.009, physical radius 1.89mm
- questions: {id, number, type, options}, without answers
- width_mm=210, height_mm=297

Two columns of 20 rows. Bubble x=.17+(column*.43)+(optionIndex*.065),
y=.25+(row*.031). QR payload is the **raw opaque backend-issued sheet token**,
not a student id, JSON object, or trustworthy identity assertion by the client.
Backend must look up that decoded token and check immutable exam release.

Recommended frontend framing guidance: downsample grayscale, adaptive/Otsu
threshold, connected dark components, dense square candidates, then extremal
quadrilateral in clockwise order. Check all four components are visible and
stable across frames. An outline ring or QR finder should not pass a solid
square density check. jsQR may decode the top-right region after perspective
rectification (or a whole-frame downsample). These are advisory UX checks only;
server independently checks all pixels and resolves orientation. Strong skew,
occlusions, folds, glare, large handwritten marks near page edges, and busy
backgrounds can legitimately reject. Retake instead of fabricating success.

## Upload and resource boundaries

Caller MUST check byte limit (4MB), JPEG/PNG format, image.DecodeConfig and
width*height <=8MP **before image.Decode**, otherwise a compressed bomb may
allocate before this package can reject it. Recognize additionally rejects nil,
sides below 500px, and over 8MP; it downsamples longest side to 1600. Serialize
local recognitions or impose a small concurrency bound; no unbounded goroutines.
QR rendering accepts 16..256 UTF-8 bytes (normal random URL-safe tokens work).
The manifest is reconstructed and compared to prevent caller-edited geometry.

Conservative checks: four-square geometry, black-white contrast >=90/255,
uniform reference patches, QR only in expected rectified box, marker edge
sharpness, minimum bubble radius 4.5px. Each bubble must retain visible printed
ring support; white occlusion/missing rings become uncertain rather than blank.
Bubble interior dark/coverage reject bands classify faint, partial or erased
marks uncertain. Single choice and
true/false multi-fill is invalid_multiple; multi-select multi-fill is answered.
These thresholds are preliminary; no real-world accuracy is claimed.

## Dependencies and tests

Dependencies are owned/added by backend, not by this package task:
github.com/makiuchi-d/gozxing v0.1.1 (mature QR decoder),
github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e (encoder).

Run: go test ./internal/vision -count=1

Tests cover explicit blank, single fill, invalid single multiple fill, valid
multiple fill, faint/mixed/partial and white-occlusion uncertain marks; synthetic
tilt, perspective, 90/180 rotation; JPEG compression and nonzero image origin;
missing markers, absent QR, blur, low contrast, image size limits,
manifest tampering, duplicate/invalid question specification, 40x5 capacity,
SVG XML validity/escaping and JSON geometry round trip. PNG synthetic geometry
is not a font-rendered SVG golden or physical print/photo test. Real-device
acceptance and broad lighting/paper/pencil testing remain outstanding.

Validation in this implementation environment: full synthetic suite PASS
(0.792s), race-detector suite PASS (10.399s), go vet PASS. Dependencies were
verified by the official Go checksum service during backend installation; tests
ran with GOPROXY=off and -mod=readonly, preserving default checksum settings.
