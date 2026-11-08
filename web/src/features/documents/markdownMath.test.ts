import { describe, it, expect } from "vitest";
import { normalizeOCRMath } from "./markdownMath";
describe("safe OCR math delimiters", () => {
  it("normalizes actual Ollama inline and display TeX", () => {
    const b = String.fromCharCode(92);
    expect(normalizeOCRMath("5. " + b + "( y = 2x + 1 " + b + ")")).toBe(
      "5. $ y = 2x + 1 $",
    );
    expect(normalizeOCRMath(b + "[x^2" + b + "]")).toBe("$$x^2$$");
  });
  it("preserves inline code and leaves HTML untrusted for skipHtml renderer", () => {
    const b = String.fromCharCode(92),
      t = String.fromCharCode(96);
    const code = t + b + "(x" + b + ")" + t;
    expect(normalizeOCRMath(code)).toBe(code);
    expect(normalizeOCRMath("<script>alert(1)</script>")).toBe(
      "<script>alert(1)</script>",
    );
  });
});
