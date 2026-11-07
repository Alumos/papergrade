import { useState, type FormEvent } from "react";
import { BookOpen, Loader2 } from "lucide-react";
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
import { Feedback } from "@/components/app/feedback";
import { post, errorText } from "@/api";
import type { Session } from "@/types";
export default function Login({
  onLogin,
  error,
  onRetry,
}: {
  onLogin: (session: Session) => void;
  error: string;
  onRetry: () => void;
}) {
  const [username, setUsername] = useState(""),
    [password, setPassword] = useState(""),
    [busy, setBusy] = useState(false),
    [message, setMessage] = useState("");
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setMessage("");
    try {
      onLogin(await post<Session>("/auth/login", { username, password }));
    } catch (e) {
      setMessage(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="grid min-h-svh place-items-center bg-background px-4 py-12">
      <div className="w-full max-w-sm space-y-7">
        <div className="flex justify-center items-center gap-2 text-xl font-semibold tracking-tight">
          <BookOpen className="size-6" />
          阅见{" "}
          <span className="text-sm font-normal text-muted-foreground">
            Papergrade
          </span>
        </div>
        <Card>
          <CardHeader>
            <CardTitle className="text-2xl">登录教师工作台</CardTitle>
            <CardDescription>
              准备考试、打印个人卡、扫描与复核。
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form id="login" onSubmit={submit} className="space-y-5">
              {(message || error) && (
                <Feedback error>
                  {message || error}
                  {error && (
                    <Button
                      className="min-h-11"
                      type="button"
                      variant="link"
                      onClick={onRetry}
                    >
                      重新连接
                    </Button>
                  )}
                </Feedback>
              )}
              <div className="space-y-2">
                <Label htmlFor="username">账户</Label>
                <Input
                  className="min-h-11"
                  id="username"
                  autoComplete="username"
                  required
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="教师账户"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">密码</Label>
                <Input
                  className="min-h-11"
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="请输入密码"
                />
              </div>
              <Button className="min-h-11 w-full" disabled={busy}>
                {busy && <Loader2 className="animate-spin" />}
                {busy ? "正在登录" : "登录"}
              </Button>
            </form>
          </CardContent>
          <CardFooter>
            <p className="text-xs leading-6 text-muted-foreground">
              账户由部署管理员创建，没有演示密码。通过同源 Cookie
              管理会话，不在本机保存密码。
            </p>
          </CardFooter>
        </Card>
        <p className="text-center text-xs text-muted-foreground">
          固定标准卡 · 云端识别 · 教师最终复核
        </p>
      </div>
    </div>
  );
}
