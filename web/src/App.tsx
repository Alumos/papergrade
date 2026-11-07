import { useEffect, useState } from "react";
import { BookOpen, LogOut, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Feedback, Loading, PageTitle } from "@/components/app/feedback";
import { api, ApiError, post, errorText } from "@/api";
import { useRemote } from "@/hooks";
import type { Session } from "@/types";
import Login from "@/features/auth/Login";
import ClassesPage from "@/features/classes/ClassesPage";
import ExamsPage from "@/features/exams/ExamsPage";
import ExamForm from "@/features/exams/ExamForm";
import ExamWorkspace from "@/features/exams/ExamWorkspace";
type Capabilities = {
  storage_mode: string;
  recognition_mode: string;
  card_recognition_configured: boolean;
  ocr_configured: boolean;
  ai_configured: boolean;
};
export default function App() {
  const [session, setSession] = useState<Session | null>(null),
    [checking, setChecking] = useState(true),
    [error, setError] = useState(""),
    [page, setPage] = useState("exams"),
    [selected, setSelected] = useState<string | null>(null),
    [create, setCreate] = useState(false),
    [epoch, setEpoch] = useState(0);
  const check = () => {
    setChecking(true);
    api<Session>("/me")
      .then(setSession)
      .catch((e) => {
        if (!(e instanceof ApiError && e.status === 401))
          setError(errorText(e));
      })
      .finally(() => setChecking(false));
  };
  useEffect(check, []);
  useEffect(() => {
    const expired = () => {
      setSession(null);
      setError("会话已过期，请重新登录，本机扫描队列保留。");
    };
    window.addEventListener("papergrade:session-expired", expired);
    return () =>
      window.removeEventListener("papergrade:session-expired", expired);
  }, []);
  const logout = async () => {
    try {
      await post("/auth/logout", {});
      setSession(null);
      setSelected(null);
      setCreate(false);
    } catch (e) {
      setError(errorText(e));
    }
  };
  if (checking)
    return (
      <div className="mx-auto max-w-lg px-6 pt-24">
        <Loading />
      </div>
    );
  if (!session)
    return (
      <Login
        error={error}
        onRetry={check}
        onLogin={(value) => {
          setSession(value);
          setError("");
        }}
      />
    );
  return (
    <div className="min-h-svh bg-background text-foreground">
      <header className="border-b">
        <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-4 sm:px-8">
          <div className="flex items-center gap-2 font-semibold tracking-tight">
            <BookOpen className="size-5" />
            阅见{" "}
            <span className="hidden text-xs font-normal text-muted-foreground sm:inline">
              Papergrade
            </span>
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden text-xs text-muted-foreground sm:inline">
              {session.user.username}
            </span>
            <Badge variant="outline">教师工作台</Badge>
            <Button
              className="min-h-11"
              variant="ghost"
              size="icon"
              aria-label="退出登录"
              onClick={() => void logout()}
            >
              <LogOut />
            </Button>
          </div>
        </div>
      </header>
      <div className="mx-auto max-w-7xl space-y-6 px-4 py-6 sm:px-8">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <Tabs
            value={page}
            onValueChange={(value) => {
              setPage(value);
              setSelected(null);
              setCreate(false);
            }}
          >
            <TabsList>
              <TabsTrigger className="min-h-11" value="exams">
                考试
              </TabsTrigger>
              <TabsTrigger className="min-h-11" value="classes">
                班级与名册
              </TabsTrigger>
            </TabsList>
          </Tabs>
          <Button
            className="min-h-11"
            variant="outline"
            onClick={() => {
              setPage("exams");
              setSelected(null);
              setCreate(true);
            }}
          >
            <Plus />
            新建考试
          </Button>
        </div>
        <CapabilitiesBanner mode={session.storage_mode} />
        <Separator />
        {error && <Feedback error>{error}</Feedback>}
        <main>
          {page === "classes" ? (
            <ClassesPage />
          ) : create ? (
            <div className="space-y-6">
              <Button
                className="min-h-11"
                variant="ghost"
                onClick={() => setCreate(false)}
              >
                ← 返回考试
              </Button>
              <PageTitle
                title="新建考试"
                description="配置客观题并发布个人答题卡。"
              />
              <ExamForm
                onSaved={(value) => {
                  setCreate(false);
                  setSelected(value.id);
                  setEpoch((v) => v + 1);
                }}
              />
            </div>
          ) : selected ? (
            <ExamWorkspace
              id={selected}
              onBack={() => {
                setSelected(null);
                setEpoch((v) => v + 1);
              }}
            />
          ) : (
            <ExamsPage
              key={epoch}
              onCreate={() => setCreate(true)}
              onSelect={setSelected}
            />
          )}
        </main>
        <Separator />
        <footer className="flex flex-wrap justify-between gap-2 py-2 text-xs text-muted-foreground">
          <span>Papergrade · 固定标准卡试验版</span>
          <span>真实 API · 仅正式入库算成功</span>
        </footer>
      </div>
    </div>
  );
}
function CapabilitiesBanner({ mode }: { mode: string }) {
  const caps = useRemote<Capabilities>("/settings/capabilities");
  return (
    <div className="space-y-3">
      {["memory", "memory_dev"].includes(caps.data?.storage_mode ?? mode) && (
        <Feedback>
          开发内存模式：重启服务会丢失数据，不可用于正式考试。
        </Feedback>
      )}
      {caps.error && <Feedback error>无法确认服务能力：{caps.error}</Feedback>}
      {caps.data && (
        <div className="flex flex-wrap gap-2">
          <Badge variant="secondary">
            {caps.data.recognition_mode === "local_experimental"
              ? "本地实验识别"
              : caps.data.card_recognition_configured
                ? "云端识别已配置"
                : "云端识别未配置"}
          </Badge>
          <Badge variant="outline">
            OCR {caps.data.ocr_configured ? "已配置" : "未配置"}
          </Badge>
          <Badge variant="outline">
            AI {caps.data.ai_configured ? "已配置" : "未配置"}
          </Badge>
        </div>
      )}
    </div>
  );
}
