import type { Point } from "./omr";
// release QR is deliberately not an input. A recognized number plus an observed
// physical departure/motion is the only v2 automatic-card transition.
export class NumberTracker {
  private lastCorners: Point[] = [];
  private moved = true;
  private lastAt = 0;
  observe(corners?: Point[]) {
    if (!corners) {
      this.moved = true;
      return;
    }
    if (
      this.lastCorners.length &&
      corners.some(
        (p, i) =>
          Math.hypot(p.x - this.lastCorners[i].x, p.y - this.lastCorners[i].y) >
          0.035,
      )
    )
      this.moved = true;
  }
  canInspect(now = Date.now()) {
    return this.moved && now - this.lastAt > 1500;
  }
  captured(corners: Point[], now = Date.now()) {
    this.lastCorners = corners.map((p) => ({ ...p }));
    this.moved = false;
    this.lastAt = now;
  }
  rearm() {
    this.moved = true;
  }
}
