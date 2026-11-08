import { describe, it, expect } from "vitest";
import {
  recognizeOMR,
  homography,
  project,
  type PixelImage,
  type Point,
} from "./omr";
import type { Manifest } from "../types";
function manifest(): Manifest {
  const m: Manifest = {
    version: "a5-v2",
    width_mm: 210,
    height_mm: 148.5,
    markers: [
      { x: 10 / 210, y: 10 / 148.5 },
      { x: 200 / 210, y: 10 / 148.5 },
      { x: 200 / 210, y: 138.5 / 148.5 },
      { x: 10 / 210, y: 138.5 / 148.5 },
    ],
    marker_size: 4 / 210,
    student_number_digits: 3,
    student_bubbles: [],
    black_reference: {
      x: 119 / 210,
      y: 30 / 148.5,
      width: 5 / 210,
      height: 5 / 148.5,
    },
    white_reference: {
      x: 128 / 210,
      y: 30 / 148.5,
      width: 5 / 210,
      height: 5 / 148.5,
    },
    questions: [
      {
        id: "q1",
        number: "1",
        type: "single_choice",
        options: ["A", "B", "C", "D"],
      },
      {
        id: "q2",
        number: "2",
        type: "multiple_choice",
        options: ["A", "B", "C", "D"],
      },
    ],
    bubbles: [],
  };
  for (let c = 0; c < 3; c++)
    for (let d = 0; d < 10; d++)
      m.student_bubbles!.push({
        column: c,
        digit: d,
        x: (22 + 5 * c) / 210,
        y: (35 + 4.5 * d) / 148.5,
        r: 1.8 / 210,
      });
  for (let i = 0; i < 2; i++)
    for (let j = 0; j < 4; j++)
      m.bubbles!.push({
        question_id: "q" + (i + 1),
        option: "ABCD"[j],
        x: (27 + 6 * j) / 210,
        y: (88 + 5 * i) / 148.5,
        r: 1.8 / 210,
      });
  return m;
}
function image(
  m: Manifest,
  digits: string,
  answers: string[][],
  corners = m.markers,
  extraDigit = false,
) {
  const width = 840,
    height = 594,
    data = new Uint8ClampedArray(width * height * 4).fill(255);
  for (let i = 3; i < data.length; i += 4) data[i] = 255;
  const H = homography(m.markers, corners);
  const put = (p: Point, r: number, value = 0) => {
    const px = p.x * width,
      py = p.y * height;
    for (let y = Math.floor(py - r); y <= Math.ceil(py + r); y++)
      for (let x = Math.floor(px - r); x <= Math.ceil(px + r); x++)
        if (
          x >= 0 &&
          y >= 0 &&
          x < width &&
          y < height &&
          Math.hypot(x - px, y - py) <= r
        ) {
          const i = (y * width + x) * 4;
          data[i] = data[i + 1] = data[i + 2] = value;
        }
  };
  for (const b of m.student_bubbles!)
    if (
      digits[b.column] === String(b.digit) ||
      (extraDigit && b.column === 0 && b.digit === 1)
    )
      put(project(H, b.x, b.y), 6);
  for (const [i, q] of m.questions!.entries())
    for (const b of m.bubbles!.filter((b) => b.question_id === q.id))
      if (answers[i]?.includes(b.option)) put(project(H, b.x, b.y), 6);
  const ref = m.black_reference!;
  for (let y = 0; y < 12; y++)
    for (let x = 0; x < 12; x++)
      put(
        project(H, ref.x + (ref.width * x) / 12, ref.y + (ref.height * y) / 12),
        1,
      );
  return { width, height, data } satisfies PixelImage;
}
describe("browser OMR projective evidence", () => {
  it("retains leading zeros and recognizes single/multiple options", () => {
    const m = manifest();
    const out = recognizeOMR(
      image(m, "007", [["B"], ["A", "C"]]),
      m,
      m.markers,
    );
    expect(out.student_number).toBe("007");
    expect(out.identity_state).toBe("recognized");
    expect(out.answers.map((a) => a.selected)).toEqual([["B"], ["A", "C"]]);
  });
  it("decodes perspective-skewed student number via manifest homography", () => {
    const m = manifest(),
      quad = [
        { x: 0.08, y: 0.11 },
        { x: 0.93, y: 0.04 },
        { x: 0.96, y: 0.91 },
        { x: 0.04, y: 0.94 },
      ];
    expect(
      recognizeOMR(image(m, "012", [["D"], []], quad), m, quad).student_number,
    ).toBe("012");
  });
  it("never guesses blank or multiple-filled identity", () => {
    const m = manifest();
    expect(recognizeOMR(image(m, "", []), m, m.markers).identity_state).toBe(
      "blank",
    );
    const out = recognizeOMR(
      image(m, "007", [], m.markers, true),
      m,
      m.markers,
    );
    expect(out.identity_state).toBe("invalid_multiple");
    expect(out.student_number).toBe("");
  });
  it("missing one digit remains uncertain and does not bind", () => {
    const m = manifest(),
      out = recognizeOMR(image(m, "00", []), m, m.markers);
    expect(out.identity_state).toBe("uncertain");
    expect(out.student_number).toBe("");
  });
});
