import type { Manifest, IdentityState, RecognizedAnswer } from "../types";
export type PixelImage = {
  width: number;
  height: number;
  data: Uint8ClampedArray;
};
export type Point = { x: number; y: number };
export type OMRPreview = {
  student_number: string;
  identity_state: IdentityState;
  answers: RecognizedAnswer[];
  corners: Point[];
  token?: string;
  quality: { contrast: number };
  message: string;
};
export function homography(from: Point[], to: Point[]) {
  const rows: number[][] = [];
  for (let i = 0; i < 4; i++) {
    const { x, y } = from[i],
      u = to[i].x,
      v = to[i].y;
    rows.push(
      [x, y, 1, 0, 0, 0, -u * x, -u * y, u],
      [0, 0, 0, x, y, 1, -v * x, -v * y, v],
    );
  }
  for (let c = 0; c < 8; c++) {
    let pivot = c;
    for (let r = c + 1; r < 8; r++)
      if (Math.abs(rows[r][c]) > Math.abs(rows[pivot][c])) pivot = r;
    if (Math.abs(rows[pivot][c]) < 1e-9) throw new Error("角点无法映射");
    [rows[c], rows[pivot]] = [rows[pivot], rows[c]];
    const n = rows[c][c];
    for (let j = c; j < 9; j++) rows[c][j] /= n;
    for (let r = 0; r < 8; r++)
      if (r !== c) {
        const k = rows[r][c];
        for (let j = c; j < 9; j++) rows[r][j] -= k * rows[c][j];
      }
  }
  return [...rows.map((r) => r[8]), 1];
}
export function project(h: number[], x: number, y: number): Point {
  const d = h[6] * x + h[7] * y + 1;
  return {
    x: (h[0] * x + h[1] * y + h[2]) / d,
    y: (h[3] * x + h[4] * y + h[5]) / d,
  };
}
function gray(image: PixelImage) {
  const result = new Uint8Array(image.width * image.height);
  for (let i = 0; i < result.length; i++) {
    const j = i * 4;
    result[i] =
      (image.data[j] * 77 + image.data[j + 1] * 150 + image.data[j + 2] * 29) >>
      8;
  }
  return result;
}
export function markerCandidates(image: PixelImage): Point[] {
  const w = image.width,
    h = image.height,
    g = gray(image),
    seen = new Uint8Array(g.length),
    stack: number[] = [],
    found: Point[] = [];
  for (let p = 0; p < g.length; p++) {
    if (seen[p] || g[p] > 75) continue;
    seen[p] = 1;
    stack.push(p);
    let n = 0,
      minX = w,
      maxX = 0,
      minY = h,
      maxY = 0;
    while (stack.length) {
      const a = stack.pop()!,
        x = a % w,
        y = Math.floor(a / w);
      n++;
      minX = Math.min(minX, x);
      maxX = Math.max(maxX, x);
      minY = Math.min(minY, y);
      maxY = Math.max(maxY, y);
      const push = (b: number) => {
        if (b >= 0 && !seen[b] && g[b] <= 75) {
          seen[b] = 1;
          stack.push(b);
        }
      };
      if (x) push(a - 1);
      if (x < w - 1) push(a + 1);
      if (y) push(a - w);
      if (y < h - 1) push(a + w);
    }
    const bw = maxX - minX + 1,
      bh = maxY - minY + 1;
    if (
      bw >= 4 &&
      bh >= 4 &&
      bw < w * 0.065 &&
      bh < h * 0.065 &&
      bw / bh > 0.65 &&
      bw / bh < 1.5 &&
      n / (bw * bh) > 0.8
    )
      found.push({ x: (minX + maxX) / 2 / w, y: (minY + maxY) / 2 / h });
  }
  return found;
}
function pickCorners(points: Point[]) {
  const ts = [
    { x: 0, y: 0 },
    { x: 1, y: 0 },
    { x: 1, y: 1 },
    { x: 0, y: 1 },
  ];
  const corners = ts.map(
    (t) =>
      points
        .filter(
          (p) => (t.x ? p.x > 0.5 : p.x < 0.5) && (t.y ? p.y > 0.5 : p.y < 0.5),
        )
        .sort(
          (a, b) =>
            Math.hypot(a.x - t.x, a.y - t.y) - Math.hypot(b.x - t.x, b.y - t.y),
        )[0],
  );
  if (corners.some((p) => !p))
    throw new Error("未找到完整四角，请只拍裁切后的一张卡");
  const area =
    Math.abs(corners[1].x - corners[0].x) *
    Math.abs(corners[3].y - corners[0].y);
  if (area < 0.2) throw new Error("答题卡过小，请靠近");
  return corners;
}
function sample(g: Uint8Array, w: number, h: number, p: Point) {
  const x = Math.round(p.x * w),
    y = Math.round(p.y * h);
  return x >= 0 && y >= 0 && x < w && y < h ? g[y * w + x] : 255;
}
function boxMean(
  g: Uint8Array,
  w: number,
  h: number,
  H: number[],
  box: { x: number; y: number; width: number; height: number },
) {
  let total = 0,
    n = 0;
  for (let yi = 1; yi < 5; yi++)
    for (let xi = 1; xi < 5; xi++) {
      total += sample(
        g,
        w,
        h,
        project(H, box.x + (box.width * xi) / 5, box.y + (box.height * yi) / 5),
      );
      n++;
    }
  return total / n;
}
export function classifyMarks(values: number[]) {
  const selected = values
    .map((v, i) => ({ v, i }))
    .filter((x) => x.v >= 0.53)
    .map((x) => x.i);
  const uncertain = values.some((v) => v > 0.2 && v < 0.53);
  return { selected, uncertain };
}
export function recognizeOMR(
  image: PixelImage,
  manifest: Manifest,
  providedCorners?: Point[],
): OMRPreview {
  if (!["a5-v2", "a5-v3", "a5-v4"].includes(manifest.version))
    throw new Error("此浏览器OMR仅支持半A4学号卡（a5-v2/v3/v4），旧卡仍由服务端识别");
  if (
    !manifest.black_reference ||
    !manifest.white_reference ||
    !manifest.student_bubbles ||
    !manifest.bubbles
  )
    throw new Error("模板缺少OMR采样区");
  const g = gray(image),
    w = image.width,
    h = image.height;
  const candidates = providedCorners ? [] : markerCandidates(image);
  const corners = providedCorners ?? pickCorners(candidates);
  if (!providedCorners) {
    const inverse = homography(corners, [
      { x: 0, y: 0 },
      { x: 1, y: 0 },
      { x: 1, y: 1 },
      { x: 0, y: 1 },
    ]);
    const extra = candidates.some((p) => {
      const q = project(inverse, p.x, p.y);
      return (
        ((Math.abs(q.x) < 0.025 || Math.abs(q.x - 1) < 0.025) &&
          q.y > 0.08 &&
          q.y < 0.92) ||
        ((Math.abs(q.y) < 0.025 || Math.abs(q.y - 1) < 0.025) &&
          q.x > 0.08 &&
          q.x < 0.92)
      );
    });
    if (extra)
      throw new Error("卡边出现额外定位块，疑似整张双联页；请裁切单卡");
  }
  let best:
    { H: number[]; black: number; white: number; corners: Point[] } | undefined;
  for (let turn = 0; turn < 4; turn++) {
    const rotated = corners.map((_, i) => corners[(i + turn) % 4]),
      H = homography(manifest.markers, rotated);
    const black = boxMean(g, w, h, H, manifest.black_reference),
      white = boxMean(g, w, h, H, manifest.white_reference);
    if (!best || white - black > best.white - best.black)
      best = { H, black, white, corners: rotated };
  }
  const { H, black, white } = best!;
  const contrast = white - black;
  if (contrast < 65)
    throw new Error("方向参考或黑白对比不可靠，请补光并只拍单卡");
  const darkness = (b: { x: number; y: number; r: number }) => {
    let total = 0,
      count = 0;
    for (let yy = -3; yy <= 3; yy++)
      for (let xx = -3; xx <= 3; xx++) {
        if (xx * xx + yy * yy > 9) continue;
        const p = project(
          H,
          b.x + (b.r * 0.62 * xx) / 3,
          b.y +
            (((b.r * manifest.width_mm) / manifest.height_mm) * 0.62 * yy) / 3,
        );
        total += Math.max(
          0,
          Math.min(1, (white - sample(g, w, h, p)) / contrast),
        );
        count++;
      }
    return total / count;
  };
  let student_number = "",
    identity_state: IdentityState = "recognized";
  let allBlank = true;
  for (
    let column = 0;
    column < (manifest.student_number_digits ?? 0);
    column++
  ) {
    const choices = manifest.student_bubbles
      .filter((b) => b.column === column)
      .sort((a, b) => a.digit - b.digit);
    const selected = classifyMarks(choices.map(darkness));
    if (selected.selected.length > 1) identity_state = "invalid_multiple";
    else if (selected.uncertain || selected.selected.length !== 1) {
      if (identity_state !== "invalid_multiple") identity_state = "uncertain";
    }
    if (selected.selected.length || selected.uncertain) allBlank = false;
    student_number +=
      selected.selected.length === 1 && !selected.uncertain
        ? String(choices[selected.selected[0]].digit)
        : "?";
  }
  if (allBlank) identity_state = "blank";
  if (identity_state !== "recognized") student_number = "";
  const answers: RecognizedAnswer[] = [];
  for (const question of manifest.questions ?? []) {
    const bubbles = manifest.bubbles.filter(
      (b) => b.question_id === question.id,
    );
    const marks = classifyMarks(bubbles.map(darkness));
    const selected = marks.selected.map((i) => bubbles[i].option);
    answers.push({
      question_id: question.id,
      selected,
      state: marks.uncertain
        ? "uncertain"
        : selected.length > 1 && question.type !== "multiple_choice"
          ? "invalid_multiple"
          : selected.length
            ? "answered"
            : "blank",
      confidence: marks.uncertain ? 0.5 : 1,
    });
  }
  return {
    student_number,
    identity_state,
    answers,
    corners: best!.corners,
    quality: { contrast },
    message:
      identity_state === "recognized"
        ? "已识别学号，等待服务器独立复核"
        : "学号未确定，需教师复核",
  };
}
