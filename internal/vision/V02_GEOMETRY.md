# a5-v2 geometry and identity contract

Legacy a4-v1 is unchanged. New BuildStudentManifest(questions,digits) accepts 1..40 questions, 1..12 digits. RenderSVG emits one 210x148.5mm card. RenderTwoUpSVG takes 1..2 PrintCard values and emits A4 210x297mm, no scale, cut line, odd lower half blank.

All x/width normalize by210mm, y/height by148.5mm. r/marker_size normalize by width. All circles radius1.8mm (diameter3.6mm). Markers centers (10,10),(200,10),(200,138.5),(10,138.5), side4mm giving8mm margin. QR box(158,25,32,32); black_reference(119,30,5,5), white_reference(128,30,5,5).

StudentBubbles JSON student_bubbles [{column,digit,x,y,r}], column zero-based, digit0..9. Physical center(22+5*column,35+4.5*digit)mm. student_number_digits exact locked roster width, never padded.

Questions four groups of10 rows. Bubble center(27+48*group+6*optionIndex,88+5*row)mm. Labels printed above each bubble font1.25mm baseline cy-2.2, never inside. No group option heading, so mixed question types retain exact labels.

V2 QR only shared opaque exam/release token, never student identity. Optional, noQR recognition valid. Recognition.StudentNumber preserves leading zeroes, IdentityState recognized/blank/uncertain/invalid_multiple, incomplete digits uncertain and no guessed number. V1 IdentityState not_required, personal QR remains mandatory legacy compatibility.

Server derives its own corner positions and unique orientation from reference patches, uses width/height ratio for circular sample Y, checks ring support and >=4.5px projected radius, rejects uncut multicard boundary-marker groups and mirror/ambiguous refs. Does not trust browser IDs/corners/answers. Caller must DecodeConfig preflight4MB/8MP before decode and serialize recognition for small VPS.

Tests: v1 regressions, v2 1/10/11/40 renders, noQR leading zeros, blank/missing/faint/multi/white occlusion, perspective, mirror and full two-card rejection. Full tests PASS1.325s, race PASS14.095s, vet PASS. Synthetic/image tests are not real-phone or physical2B print acceptance; labels are intentionally small and require physical review.
