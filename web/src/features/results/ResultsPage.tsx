import { useState } from "react";
import UnboundReview from "./UnboundReview";
import RegradeDialog from "./RegradeDialog";
import { Download, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableHeader,
  TableRow,
  TableHead,
  TableBody,
  TableCell,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import { Feedback, Loading, Empty, Status } from "@/components/app/feedback";
import { useRemote } from "@/hooks";
import { post, errorText } from "@/api";
import { score } from "@/lib";
import type { Exam, Result, Results } from "@/types";
export default function ResultsPage({
  exam,
  onChanged,
}: {
  exam: Exam;
  onChanged: (exam: Exam) => void;
}) {
  const capabilities = useRemote<{ regrade_available: boolean }>(
    "/settings/capabilities",
  );
  const results = useRemote<Results>("/exams/" + exam.id + "/results"),
    reviews = useRemote<{ items: Result[] }>("/exams/" + exam.id + "/reviews");
  const [tab, setTab] = useState("all"),
    [review, setReview] = useState<Result | null>(null),
    [answers, setAnswers] = useState<Record<string, string[]>>({}),
    [reason, setReason] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const open = (item: Result) => {
    setReview(item);
    setAnswers(
      Object.fromEntries(
        exam.questions.map((q) => [
          q.id!,
          item.answers?.find((a) => a.question_id === q.id)?.selected ?? [],
        ]),
      ),
    );
    setReason("");
    setError("");
  };
  const save = async () => {
    if (!review) return;
    setBusy(true);
    setError("");
    try {
      await post("/results/" + review.id + "/review", {
        revision: review.revision,
        answers: Object.entries(answers).map(([question_id, selected]) => ({
          question_id,
          selected,
        })),
        reason: reason.trim(),
      });
      setReview(null);
      results.reload();
      reviews.reload();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  const summary = results.data?.summary,
    items = tab === "review" ? reviews.data?.items : results.data?.items;
  return (
    <div className="space-y-5">
      {!!exam.student_number_digits && (
        <UnboundReview
          exam={exam}
          onBound={() => {
            results.reload();
            reviews.reload();
          }}
        />
      )}
      <p className="text-xs text-muted-foreground">
        考试 revision {exam.revision} · 当前评分版本{" "}
        {exam.grading_version?.slice(0, 12) ?? "原始发布答案"} ·
        原发布卡片保持不变
      </p>
      <div className="flex flex-wrap justify-end gap-2">
        {capabilities.data?.regrade_available && (
          <RegradeDialog
            exam={exam}
            onComplete={(value) => {
              onChanged(value);
              results.reload();
              reviews.reload();
            }}
          />
        )}
        <Button
          className="min-h-11"
          variant="outline"
          onClick={() => {
            results.reload();
            reviews.reload();
          }}
        >
          <RefreshCw />
          刷新
        </Button>
        <Button className="min-h-11" asChild>
          <a href={"/api/v1/exams/" + exam.id + "/export.csv"} download>
            <Download />
            导出 CSV
          </a>
        </Button>
      </div>
      {(error || results.error || reviews.error) && (
        <Feedback error>{error || results.error || reviews.error}</Feedback>
      )}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        {[
          ["已收", summary?.received],
          ["待复核", summary?.needs_review],
          ["未收", summary?.missing],
          ["平均分", score(summary?.average_units)],
        ].map(([label, value]) => (
          <Card key={label}>
            <CardHeader>
              <CardDescription>{label}</CardDescription>
              <CardTitle className="text-3xl tabular-nums">
                {value ?? "—"}
              </CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>
      <Card>
        <CardHeader>
          <CardTitle>答卷结果</CardTitle>
          <Tabs value={tab} onValueChange={setTab}>
            <TabsList>
              <TabsTrigger className="min-h-11" value="all">
                全部结果
              </TabsTrigger>
              <TabsTrigger className="min-h-11" value="review">
                待复核 ({reviews.data?.items.length ?? 0})
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </CardHeader>
        <CardContent>
          {results.loading || reviews.loading ? (
            <Loading />
          ) : !items?.length ? (
            <Empty>
              暂无{tab === "review" ? "待复核答卷" : "答卷，请先扫描个人卡"}。
            </Empty>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>学生</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>得分</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>
                      <p className="font-medium">{item.student_name}</p>
                      <p className="text-xs text-muted-foreground">
                        {item.student_number}
                      </p>
                    </TableCell>
                    <TableCell>
                      <Status value={item.status} />
                    </TableCell>
                    <TableCell>
                      {score(item.score_units)} / {score(item.max_score_units)}
                    </TableCell>
                    <TableCell>
                      <Button
                        className="min-h-11"
                        variant="ghost"
                        onClick={() => open(item)}
                      >
                        核对答案
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      <Feedback>
        异常多填、空白和不确定答案请核对原图。复核提交答案与原因，服务端计分并保留
        revision；重评仅在服务端启用时显示，必须明确确认并记录原因。
      </Feedback>
      <Dialog open={!!review} onOpenChange={(v) => !v && setReview(null)}>
        <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{review?.student_name} · 答卷复核</DialogTitle>
            <DialogDescription>
              学号 {review?.student_number} · 版本 {review?.revision}
              ，核对后由服务器重新计分。
            </DialogDescription>
          </DialogHeader>
          {review && (
            <>
              <a
                className="text-sm underline"
                href={"/api/v1/captures/" + review.capture_id + "/image"}
                target="_blank"
                rel="noreferrer"
              >
                打开原始答卷 ↗
              </a>
              <img
                className="max-h-72 w-full rounded-lg border object-contain"
                src={"/api/v1/captures/" + review.capture_id + "/image"}
                alt="待复核答卷"
              />
              {exam.questions.map((q) => (
                <Card key={q.id}>
                  <CardContent className="space-y-3 pt-4">
                    <div className="flex justify-between">
                      <Label>第 {q.number} 题</Label>
                      <Badge variant="secondary">
                        {review.answers?.find((a) => a.question_id === q.id)
                          ?.state ?? "未识别"}
                      </Badge>
                    </div>
                    <div className="flex flex-wrap gap-4">
                      {q.options.map((option) => (
                        <Label className="flex items-center gap-2" key={option}>
                          <Checkbox
                            checked={answers[q.id!]?.includes(option) ?? false}
                            onCheckedChange={(v) =>
                              setAnswers((prev) => ({
                                ...prev,
                                [q.id!]: v
                                  ? q.type === "multiple_choice"
                                    ? [...(prev[q.id!] ?? []), option]
                                    : [option]
                                  : (prev[q.id!] ?? []).filter(
                                      (a) => a !== option,
                                    ),
                              }))
                            }
                          />
                          {option === "T"
                            ? "正确"
                            : option === "F"
                              ? "错误"
                              : option}
                        </Label>
                      ))}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      标准答案 {q.answer.join("、")} · {score(q.score_units)} 分
                    </p>
                  </CardContent>
                </Card>
              ))}
              <div className="space-y-2">
                <Label htmlFor="review-reason">复核原因</Label>
                <Textarea
                  id="review-reason"
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  placeholder="说明核对依据，保留变更记录"
                />
              </div>
              {error && <Feedback error>{error}</Feedback>}
              <DialogFooter>
                <Button
                  className="min-h-11"
                  variant="outline"
                  onClick={() => setReview(null)}
                >
                  取消
                </Button>
                <Button
                  className="min-h-11"
                  disabled={busy || !reason.trim()}
                  onClick={() => void save()}
                >
                  {busy ? "提交中" : "确认复核并计分"}
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
