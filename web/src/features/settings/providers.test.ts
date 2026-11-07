import { afterEach, describe, expect, it, vi } from "vitest";
import {
  draftFromSettings,
  providerPayload,
  providerSaveError,
  saveProviderSettings,
  testProviderSettings,
} from "./providers";
import { ApiError } from "../../api";
afterEach(() => vi.unstubAllGlobals());
const settings = {
  revision: 4,
  source: "database" as const,
  encryption_ready: true,
  storage_mode: "postgres",
  ocr: { url: "https://ocr.example.test", has_secret: true, configured: true },
  ai: {
    base_url: "https://ai.example.test/v1",
    model: "model-1",
    has_secret: true,
    configured: true,
  },
  card: {
    url: "https://card.example.test",
    has_secret: false,
    configured: true,
  },
};
describe("provider settings contract", () => {
  it("maps脱敏GET to empty secret drafts and preserves exact PUT revision shape", () => {
    const draft = draftFromSettings(settings);
    expect(draft.ocr.secret).toBe("");
    expect(draft.ai.model).toBe("model-1");
    const payload = providerPayload(settings.revision, draft);
    expect(payload).toMatchObject({
      revision: 4,
      ocr: { url: "https://ocr.example.test", secret: "" },
      ai: { base_url: "https://ai.example.test/v1", model: "model-1" },
      card: { url: "https://card.example.test", secret: "" },
    });
    expect(payload).not.toHaveProperty("providers");
  });
  it("preserves existing secrets when fields are blank and supports explicit clear", () => {
    const draft = draftFromSettings(settings);
    expect(providerPayload(4, draft).ocr.clear_secret).toBe(false);
    draft.ocr.clear_secret = true;
    const payload = providerPayload(4, draft);
    expect(payload.ocr).toMatchObject({ secret: "", clear_secret: true });
  });
  it("rejects secret clear plus non-empty secret and embedded credentials", () => {
    const draft = draftFromSettings(settings);
    draft.ocr.clear_secret = true;
    draft.ocr.secret = "new-secret";
    expect(() => providerPayload(4, draft)).toThrow("不能同时填写");
    draft.ocr.clear_secret = false;
    draft.ocr.secret = "";
    draft.ocr.url = "https://user:pass@example.test";
    expect(() => providerPayload(4, draft)).toThrow("不能内嵌凭据");
  });
});
describe("provider request boundaries (unit HTTP fixtures only)", () => {
  it("sends only saved provider selector for connection tests, never draft secrets", async () => {
    const fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          provider: "ai",
          ok: false,
          status_code: 401,
          message: "authentication failed",
          revision: 4,
        }),
        { status: 200 },
      ),
    );
    vi.stubGlobal("fetch", fetch);
    const result = await testProviderSettings("ai");
    expect(fetch.mock.calls[0][0]).toBe("/api/v1/settings/providers/test");
    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({ provider: "ai" });
    expect(result.ok).toBe(false);
    expect(result.status_code).toBe(401);
  });
  it("retains caller draft and revision on HTTP409, without silent retry or clear", async () => {
    const draft = draftFromSettings(settings);
    draft.ai.secret = "unit-only-new-key";
    draft.ai.model = "edited-model";
    const before = structuredClone(draft);
    const fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          code: "CONFLICT",
          message: "stale provider settings revision",
        }),
        { status: 409 },
      ),
    );
    vi.stubGlobal("fetch", fetch);
    await expect(saveProviderSettings(4, draft)).rejects.toMatchObject({
      status: 409,
      code: "CONFLICT",
    });
    expect(draft).toEqual(before);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(JSON.parse(fetch.mock.calls[0][1].body)).toMatchObject({
      revision: 4,
      ai: { secret: "unit-only-new-key", model: "edited-model" },
    });
  });
  it("propagates unavailable settings rather than inventing successful test results", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            code: "PROVIDER_NOT_CONFIGURED",
            message: "selected provider is not configured",
          }),
          { status: 503 },
        ),
      ),
    );
    await expect(testProviderSettings("card")).rejects.toMatchObject({
      status: 503,
      code: "PROVIDER_NOT_CONFIGURED",
    });
  });
});
it("shows actionable conflict copy without exposing server detail", () => {
  const message = providerSaveError(new ApiError(409, "CONFLICT", "stale"));
  expect(message).toContain("输入已保留");
  expect(message).toContain("关闭确认框");
  expect(message).toContain("手动合并");
  expect(providerSaveError(new Error("保存失败"))).toBe("保存失败");
});
