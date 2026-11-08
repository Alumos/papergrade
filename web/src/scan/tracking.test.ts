import { describe, it, expect } from "vitest";
import { NumberTracker } from "./tracking";
import { StabilityGate } from "./quality";
describe("v2换卡不是release QR身份锁", () => {
  it("same release QR two numbers both eligible", () => {
    const gate = new StabilityGate();
    const corners = [
      { x: 0.1, y: 0.1 },
      { x: 0.9, y: 0.1 },
      { x: 0.9, y: 0.9 },
      { x: 0.1, y: 0.9 },
    ];
    const f = (n: string) => ({
      token: "same-release",
      trackingKey: "number:" + n,
      ready: true,
      corners,
      message: "",
      brightness: 130,
      sharpness: 15,
    });
    gate.reserve("number:001");
    for (let i = 0; i < 3; i++) expect(gate.inspect(f("001"))).toBe(false);
    gate.inspect(f("002"));
    gate.inspect(f("002"));
    expect(gate.inspect(f("002"))).toBe(true);
  });
  it("requires observed movement and cooldown after selected card", () => {
    const t = new NumberTracker(),
      c = [
        { x: 0.1, y: 0.1 },
        { x: 0.9, y: 0.1 },
        { x: 0.9, y: 0.9 },
        { x: 0.1, y: 0.9 },
      ];
    expect(t.canInspect(2000)).toBe(true);
    t.captured(c, 2000);
    t.observe(c);
    expect(t.canInspect(5000)).toBe(false);
    t.observe(undefined);
    expect(t.canInspect(3000)).toBe(false);
    expect(t.canInspect(5000)).toBe(true);
  });
});
