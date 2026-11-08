import jsQR from "jsqr";
export type Point = {
  x: number;
  y: number;
};
export type FrameAnalysis = {
  token?: string;
  trackingKey?: string;
  ready: boolean;
  message: string;
  corners?: Point[];
  brightness: number;
  sharpness: number;
};
// Runs at <= 2 Hz on a small working canvas; never store video frames in React.
export function analyzeFrame(
  image: ImageData,
  requireQR = true,
): FrameAnalysis {
  const { width: w, height: h, data } = image;
  const gray = new Uint8Array(w * h);
  let sum = 0,
    edges = 0;
  for (let i = 0; i < gray.length; i++) {
    const k = i * 4;
    gray[i] = (data[k] * 77 + data[k + 1] * 150 + data[k + 2] * 29) >> 8;
    sum += gray[i];
  }
  const brightness = sum / gray.length;
  for (let y = 1; y < h - 1; y += 2)
    for (let x = 1; x < w - 1; x += 2) {
      const p = y * w + x;
      edges += Math.abs(
        4 * gray[p] - gray[p - 1] - gray[p + 1] - gray[p - w] - gray[p + w],
      );
    }
  const sharpness = edges / ((w * h) / 4);
  const seen = new Uint8Array(w * h);
  const candidates: Point[] = [];
  const stack: number[] = [];
  for (let p = 0; p < gray.length; p++) {
    if (seen[p] || gray[p] > 65) continue;
    seen[p] = 1;
    stack.push(p);
    let count = 0,
      minX = w,
      maxX = 0,
      minY = h,
      maxY = 0;
    while (stack.length) {
      const a = stack.pop()!;
      count++;
      const x = a % w,
        y = Math.floor(a / w);
      minX = Math.min(minX, x);
      maxX = Math.max(maxX, x);
      minY = Math.min(minY, y);
      maxY = Math.max(maxY, y);
      for (const b of [
        x > 0 ? a - 1 : -1,
        x < w - 1 ? a + 1 : -1,
        y > 0 ? a - w : -1,
        y < h - 1 ? a + w : -1,
      ])
        if (b >= 0 && !seen[b] && gray[b] <= 65) {
          seen[b] = 1;
          stack.push(b);
        }
    }
    const bw = maxX - minX + 1,
      bh = maxY - minY + 1;
    if (
      bw >= 3 &&
      bh >= 3 &&
      bw <= w * 0.09 &&
      bh <= h * 0.09 &&
      bw / bh > 0.5 &&
      bw / bh < 1.8 &&
      count / (bw * bh) > 0.72
    )
      candidates.push({ x: (minX + maxX) / 2 / w, y: (minY + maxY) / 2 / h });
  }
  const targets = [
    { x: 0, y: 0 },
    { x: 1, y: 0 },
    { x: 1, y: 1 },
    { x: 0, y: 1 },
  ];
  const corners = targets.map(
    (t) =>
      candidates
        .filter(
          (p) => (t.x ? p.x > 0.5 : p.x < 0.5) && (t.y ? p.y > 0.5 : p.y < 0.5),
        )
        .sort(
          (a, b) =>
            Math.hypot(a.x - t.x, a.y - t.y) - Math.hypot(b.x - t.x, b.y - t.y),
        )[0],
  );
  const qr = jsQR(data, w, h, { inversionAttempts: "dontInvert" });
  const token = qr?.data || undefined;
  const framed =
    corners.every(Boolean) &&
    Math.abs((corners[1].x - corners[0].x) * (corners[3].y - corners[0].y)) >
      0.22;
  const message =
    brightness < 65
      ? "光线偏暗，请补光"
      : brightness > 254
        ? "画面过亮，请避开反光"
        : !framed
          ? "请让四角黑色标记完整进入取景框"
          : sharpness < 3
            ? "画面模糊，请稍微抬高手机并对焦"
            : requireQR && !token
              ? "正在读取旧版个人二维码，请保持无遮挡"
              : "已找到单卡，请保持稳定";
  return {
    token,
    ready:
      brightness >= 65 &&
      brightness <= 254 &&
      framed &&
      sharpness >= 3 &&
      (!requireQR || !!token),
    message,
    corners: framed ? corners : undefined,
    brightness,
    sharpness,
  };
}
export class StabilityGate {
  private token = "";
  private previous: Point[] = [];
  private consecutive = 0;
  private captured = new Set<string>();
  constructor(tokens: string[] = []) {
    tokens.forEach((t) => this.captured.add(t));
  }
  inspect(frame: FrameAnalysis) {
    if (!frame.ready || !(frame.trackingKey ?? frame.token) || !frame.corners) {
      this.consecutive = 0;
      return false;
    }
    const moved =
      this.previous.length !== 4 ||
      frame.corners.some(
        (p, i) =>
          Math.hypot(p.x - this.previous[i].x, p.y - this.previous[i].y) >
          0.018,
      );
    const key = frame.trackingKey ?? frame.token!;
    this.consecutive = key === this.token && !moved ? this.consecutive + 1 : 1;
    this.token = key;
    this.previous = frame.corners;
    return this.consecutive >= 3 && !this.captured.has(key);
  }
  reserve(token: string) {
    this.captured.add(token);
    this.consecutive = 0;
  }
  release(token: string) {
    this.captured.delete(token);
    this.consecutive = 0;
  }
  has(token: string) {
    return this.captured.has(token);
  }
}
