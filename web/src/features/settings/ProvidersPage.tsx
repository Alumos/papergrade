import { useEffect, useState } from "react";
import {
  Save,
  RefreshCw,
  FlaskConical,
  ShieldCheck,
  KeyRound,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { Feedback, Loading, PageTitle } from "@/components/app/feedback";
import { errorText } from "@/api";
import {
  draftFromSettings,
  getProviderSettings,
  saveProviderSettings,
  testProviderSettings,
  providerNames,
  providerSaveError,
  type ProviderDraft,
  type ProviderKind,
  type ProviderSettings,
  type ProviderTest,
} from "./providers";
export default function ProvidersPage({ onSaved }: { onSaved: () => void }) {
  const [settings, setSettings] = useState<ProviderSettings | null>(null),
    [draft, setDraft] = useState<ProviderDraft | null>(null),
    [loading, setLoading] = useState(true),
    [busy, setBusy] = useState(false),
    [testing, setTesting] = useState<ProviderKind | null>(null),
    [testTarget, setTestTarget] = useState<ProviderKind | null>(null),
    [results, setResults] = useState<
      Partial<Record<ProviderKind, ProviderTest>>
    >({}),
    [error, setError] = useState(""),
    [message, setMessage] = useState(""),
    [confirm, setConfirm] = useState(false),
    [dirty, setDirty] = useState(false);
  const accept = (value: ProviderSettings) => {
    setSettings(value);
    setDraft(draftFromSettings(value));
    setDirty(false);
    setResults({});
  };
  const load = async () => {
    setLoading(true);
    setError("");
    try {
      accept(await getProviderSettings());
    } catch (e) {
      setError(errorText(e));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    void load();
  }, []);
  const change = (
    kind: ProviderKind,
    field: string,
    value: string | boolean,
  ) => {
    setDraft((prev) =>
      prev ? { ...prev, [kind]: { ...prev[kind], [field]: value } } : prev,
    );
    setDirty(true);
    setMessage("");
    setResults({});
  };
  const save = async () => {
    if (!settings || !draft) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const value = await saveProviderSettings(settings.revision, draft);
      accept(value);
      setConfirm(false);
      setMessage(
        "配置已保存，API 与 Worker 后续请求使用最新配置。密钥输入已清空，未回显已存密钥。",
      );
      onSaved();
    } catch (e) {
      setError(providerSaveError(e));
    } finally {
      setBusy(false);
    }
  };
  const test = async () => {
    if (!testTarget) return;
    const kind = testTarget;
    setTesting(kind);
    setTestTarget(null);
    setError("");
    try {
      const result = await testProviderSettings(kind);
      setResults((prev) => ({ ...prev, [kind]: result }));
    } catch (e) {
      setError(errorText(e));
    } finally {
      setTesting(null);
    }
  };
  if (loading) return <Loading />;
  if (!settings || !draft)
    return (
      <div className="space-y-4">
        <PageTitle
          title="模型与服务设置"
          description="从服务端读取脱敏配置，不在浏览器存储密钥。"
        />
        <Feedback error>{error || "配置不可用"}</Feedback>
        <Button
          className="min-h-11"
          variant="outline"
          onClick={() => void load()}
        >
          <RefreshCw />
          重新读取
        </Button>
      </div>
    );
  return (
    <div className="space-y-6">
      <PageTitle
        title="模型与服务设置"
        description="配置真实 OCR、OpenAI-compatible AI 和答题卡网关。"
        action={
          <Button
            className="min-h-11"
            variant="outline"
            disabled={busy || !!testing}
            onClick={() => {
              if (
                !dirty ||
                window.confirm("重新读取会丢弃未保存的修改，继续？")
              )
                void load();
            }}
          >
            <RefreshCw />
            重新读取
          </Button>
        }
      />
      {error && (
        <Feedback error>
          {error} 若配置版本冲突，请重新读取，不会自动覆盖他人修改。
        </Feedback>
      )}
      {message && <Feedback>{message}</Feedback>}
      {settings.warning && <Feedback>{settings.warning}</Feedback>}
      {!settings.encryption_ready && (
        <Feedback error>
          服务端未就绪加密存储。请先由部署管理员配置
          APP_ENCRYPTION_KEY；本页不会伪称密钥已安全持久化。
        </Feedback>
      )}
      <div className="flex flex-wrap gap-2">
        <Badge variant="outline">版本 {settings.revision}</Badge>
        <Badge variant="secondary">
          {settings.source === "database" ? "后台配置" : "环境引导配置"}
        </Badge>
        <Badge variant="outline">{settings.storage_mode}</Badge>
        <Badge variant="outline">
          <ShieldCheck className="mr-1 size-3" />
          {settings.encryption_ready ? "服务端加密就绪" : "加密未就绪"}
        </Badge>
        {dirty && <Badge variant="secondary">存在未保存修改</Badge>}
      </div>
      <Tabs defaultValue="ocr" className="space-y-5">
        <TabsList className="h-auto flex-wrap">
          <TabsTrigger className="min-h-11" value="ocr">
            OCR 网关
          </TabsTrigger>
          <TabsTrigger className="min-h-11" value="ai">
            AI 模型
          </TabsTrigger>
          <TabsTrigger className="min-h-11" value="card">
            答题卡网关
          </TabsTrigger>
        </TabsList>
        {(["ocr", "ai", "card"] as const).map((kind) => (
          <TabsContent key={kind} value={kind}>
            <Card>
              <CardHeader>
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <CardTitle>{providerNames[kind]}</CardTitle>
                  <Badge
                    variant={
                      settings[kind].configured ? "default" : "secondary"
                    }
                  >
                    {settings[kind].configured ? "已配置" : "未配置"}
                  </Badge>
                </div>
                <CardDescription>
                  {kind === "ocr"
                    ? "专用文档网关，负责 PDF / 图片文字提取。"
                    : kind === "ai"
                      ? "兼容 OpenAI 的分析接口；填写 API 根地址与真实可用模型名。"
                      : "专用结构化答题卡网关，返回个人 token 与客观题选项；不是普通聊天模型。"}
                  状态来自已保存配置，不代表连通或识别准确。
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-5">
                <div className="space-y-2">
                  <Label htmlFor={"provider-url-" + kind}>
                    {kind === "ai" ? "API Base URL" : "网关 URL"}
                  </Label>
                  <Input
                    className="min-h-11"
                    id={"provider-url-" + kind}
                    type="url"
                    autoComplete="off"
                    value={kind === "ai" ? draft.ai.base_url : draft[kind].url}
                    placeholder={
                      kind === "ai"
                        ? "https://api.example.com/v1"
                        : "https://gateway.example.com/endpoint"
                    }
                    disabled={busy || !!testing}
                    onChange={(e) =>
                      change(
                        kind,
                        kind === "ai" ? "base_url" : "url",
                        e.target.value,
                      )
                    }
                  />
                  <p className="text-xs text-muted-foreground">
                    留空并保存可禁用此服务。URL
                    不得包含内嵌密码，请使用独立密钥字段。
                  </p>
                </div>
                {kind === "ai" && (
                  <div className="space-y-2">
                    <Label htmlFor="provider-model-ai">模型名称</Label>
                    <Input
                      className="min-h-11"
                      id="provider-model-ai"
                      value={draft.ai.model}
                      disabled={busy || !!testing}
                      autoComplete="off"
                      onChange={(e) => change("ai", "model", e.target.value)}
                      placeholder="供应商实际提供的模型 ID"
                    />
                  </div>
                )}
                <Separator />
                <div className="space-y-2">
                  <Label htmlFor={"provider-secret-" + kind}>
                    <KeyRound className="mr-1 inline size-4" />
                    API 密钥
                  </Label>
                  <Input
                    className="min-h-11"
                    id={"provider-secret-" + kind}
                    type="password"
                    value={draft[kind].secret}
                    disabled={busy || !!testing || draft[kind].clear_secret}
                    autoComplete="new-password"
                    spellCheck={false}
                    placeholder={
                      settings[kind].has_secret
                        ? "已存密钥不回显，留空保留"
                        : "尚未保存密钥"
                    }
                    onChange={(e) => change(kind, "secret", e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    {settings[kind].has_secret
                      ? "服务端已有密钥。"
                      : "服务端尚无密钥。"}
                    空白输入表示保留，不会清空；不使用 localStorage 保存。
                  </p>
                </div>
                <Label className="flex min-h-11 items-center gap-3">
                  <Checkbox
                    checked={draft[kind].clear_secret}
                    disabled={busy || !!testing}
                    onCheckedChange={(v) => {
                      if (v) change(kind, "secret", "");
                      change(kind, "clear_secret", v === true);
                    }}
                  />
                  保存时明确清除已存密钥
                </Label>
                {results[kind] && (
                  <Feedback error={!results[kind]!.ok}>
                    {providerNames[kind]}：
                    {results[kind]!.ok ? "连通检查通过" : "连通检查未通过"} ·
                    版本 {results[kind]!.revision}
                    {results[kind]!.status_code
                      ? " · HTTP " + results[kind]!.status_code
                      : ""}
                    。{results[kind]!.message}
                  </Feedback>
                )}
              </CardContent>
              <CardFooter className="flex-wrap gap-3">
                <Button
                  className="min-h-11"
                  variant="outline"
                  disabled={busy || !!testing || dirty}
                  onClick={() => setTestTarget(kind)}
                >
                  <FlaskConical />
                  {testing === kind ? "正在测试" : "测试已保存配置"}
                </Button>
                <p className="text-xs text-muted-foreground">
                  {dirty
                    ? "先保存或重新读取，避免测试与编辑值混淆。"
                    : "仅网关 HEAD / AI models 查询，不等于识别或推理成功。"}
                </p>
              </CardFooter>
            </Card>
          </TabsContent>
        ))}
      </Tabs>
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-4">
        <p className="max-w-xl text-xs leading-6 text-muted-foreground">
          保存统一提交三项配置与
          revision。空密钥保留、显式勾选才清除。变更将用于之后的 API / Worker
          请求；测试只检查已保存值。
        </p>
        <Button
          className="min-h-11"
          disabled={busy || !!testing || !dirty}
          onClick={() => setConfirm(true)}
        >
          <Save />
          {busy ? "保存中" : "保存设置"}
        </Button>
      </div>
      <Dialog open={confirm} onOpenChange={(v) => !busy && setConfirm(v)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认保存模型设置？</DialogTitle>
            <DialogDescription>
              基于配置版本 {settings.revision}{" "}
              保存。此操作会影响后续识别和解析请求；不会回显任何密钥。
            </DialogDescription>
          </DialogHeader>
          {error && <Feedback error>{error}</Feedback>}
          {(["ocr", "ai", "card"] as const).some(
            (k) => draft[k].clear_secret,
          ) && (
            <Feedback error>
              将清除：
              {(["ocr", "ai", "card"] as const)
                .filter((k) => draft[k].clear_secret)
                .map((k) => providerNames[k])
                .join("、")}{" "}
              的密钥。
            </Feedback>
          )}
          <DialogFooter>
            <Button
              className="min-h-11"
              variant="outline"
              disabled={busy}
              onClick={() => setConfirm(false)}
            >
              取消
            </Button>
            <Button
              className="min-h-11"
              disabled={busy}
              onClick={() => void save()}
            >
              确认保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!testTarget}
        onOpenChange={(v) => !v && setTestTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              测试 {testTarget ? providerNames[testTarget] : ""}
            </DialogTitle>
            <DialogDescription>
              使用服务器已保存的配置（版本 {settings.revision}
              ）发出真实网络请求。网关使用 HEAD，AI 查询
              models，不发送试卷或执行收费推理，但供应商可能记录请求。通过不保证识别效果。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              className="min-h-11"
              variant="outline"
              onClick={() => setTestTarget(null)}
            >
              取消
            </Button>
            <Button className="min-h-11" onClick={() => void test()}>
              确认测试已保存配置
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
