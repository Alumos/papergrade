import { api, post, ApiError, errorText } from "@/api";

export function providerSaveError(error: unknown): string {
  if (error instanceof ApiError && error.status === 409) {
    return "配置版本冲突：当前输入已保留，未自动重试或覆盖。请先关闭确认框并记录拟改值，再重新读取最新配置、手动合并后保存。";
  }
  return errorText(error);
}

export type ProviderKind = "ocr" | "ai" | "card";
export type GatewaySettings = {
  url: string;
  has_secret: boolean;
  configured: boolean;
};
export type ProviderSettings = {
  revision: number;
  source: "database" | "environment";
  encryption_ready: boolean;
  storage_mode: string;
  warning?: string;
  ocr: GatewaySettings & { type?: "gateway" | "ollama"; model?: string };
  ai: {
    base_url: string;
    model: string;
    has_secret: boolean;
    configured: boolean;
  };
  card: GatewaySettings;
};
export type ProviderDraft = {
  ocr: {
    type: "gateway" | "ollama";
    model: string;
    url: string;
    secret: string;
    clear_secret: boolean;
  };
  ai: {
    base_url: string;
    model: string;
    secret: string;
    clear_secret: boolean;
  };
  card: { url: string; secret: string; clear_secret: boolean };
};
export type ProviderTest = {
  provider: ProviderKind;
  ok: boolean;
  status_code?: number;
  message: string;
  revision: number;
};
export const providerNames: Record<ProviderKind, string> = {
  ocr: "OCR 文档网关",
  ai: "AI 题目分析",
  card: "答题卡识别网关",
};
export function draftFromSettings(settings: ProviderSettings): ProviderDraft {
  return {
    ocr: {
      type: settings.ocr.type ?? "gateway",
      model: settings.ocr.model ?? "",
      url: settings.ocr.url,
      secret: "",
      clear_secret: false,
    },
    ai: {
      base_url: settings.ai.base_url,
      model: settings.ai.model,
      secret: "",
      clear_secret: false,
    },
    card: { url: settings.card.url, secret: "", clear_secret: false },
  };
}
function validateURL(label: string, value: string) {
  if (!value) return;
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new Error(label + " URL 格式无效");
  }
  if (
    !["https:", "http:"].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  ) {
    throw new Error(label + " URL 必须为 HTTP(S)，且不能内嵌凭据");
  }
}
export function providerPayload(revision: number, draft: ProviderDraft) {
  for (const kind of ["ocr", "ai", "card"] as const) {
    const value = kind === "ai" ? draft.ai.base_url : draft[kind].url;
    validateURL(providerNames[kind], value);
    if (draft[kind].clear_secret && draft[kind].secret)
      throw new Error("不能同时填写并清除密钥");
  }
  return {
    revision,
    ocr: draft.ocr,
    ai: draft.ai,
    card: draft.card,
  };
}
export const getProviderSettings = () =>
  api<ProviderSettings>("/settings/providers");
export const saveProviderSettings = (revision: number, draft: ProviderDraft) =>
  api<ProviderSettings>("/settings/providers", {
    method: "PUT",
    body: JSON.stringify(providerPayload(revision, draft)),
  });
export const testProviderSettings = (provider: ProviderKind) =>
  post<ProviderTest>("/settings/providers/test", { provider });
