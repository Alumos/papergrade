import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Textarea } from "@/components/ui/textarea";
import { Feedback } from "@/components/app/feedback";
import { post, errorText } from "@/api";
import type { Exam } from "@/types";
export default function RegradeDialog({
  exam,
  onComplete,
}: {
  exam: Exam;
  onComplete: (exam: Exam) => void;
}) {
  const [open, setOpen] = useState(false),
    [answers, setAnswers] = useState<Record<string, string[]>>({}),
    [reason, setReason] = useState(""),
    [confirmed, setConfirmed] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const start = () => {
    setAnswers(
      Object.fromEntries(exam.questions.map((q) => [q.id!, q.answer])),
    );
    setReason("");
    setConfirmed(false);
    setError("");
    setOpen(true);
  };
  const submit = async () => {
    setBusy(true);
    setError("");
    try {
      const result = await post<{
        exam: Exam;
        updated_results: number;
        grading_version: string;
      }>("/exams/" + exam.id + "/regrade", {
        revision: exam.revision,
        answers: exam.questions.map((q) => ({
          question_id: q.id!,
          answer: answers[q.id!],
        })),
        reason: reason.trim(),
      });
      onComplete(result.exam);
      setOpen(false);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <Button className="min-h-11" variant="outline" onClick={start}>
        修改答案并重评
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[90svh] overflow-auto sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>显式答案重评</DialogTitle>
            <DialogDescription>
              当前考试 revision {exam.revision}
              。保持原发布模板与个人卡，创建新评分版本并保留历史；仅对客观题重新计分，不会自动把不确定答卷标成正常。
            </DialogDescription>
          </DialogHeader>
          {error && <Feedback error>{error}</Feedback>}
          {exam.questions.map((q) => (
            <div key={q.id} className="space-y-3 rounded-lg border p-4">
              <Label>第 {q.number} 题</Label>
              <p className="text-xs text-muted-foreground">
                原答案 {q.answer.join("、")} → 新答案{" "}
                {answers[q.id!]?.join("、") || "未配置"}
              </p>
              <div className="flex flex-wrap gap-4">
                {q.options.map((option) => (
                  <Label
                    key={option}
                    className="flex min-h-11 items-center gap-2"
                  >
                    <Checkbox
                      checked={answers[q.id!]?.includes(option) ?? false}
                      onCheckedChange={(v) =>
                        setAnswers((prev) => ({
                          ...prev,
                          [q.id!]: v
                            ? q.type === "multiple_choice"
                              ? [...(prev[q.id!] ?? []), option]
                              : [option]
                            : (prev[q.id!] ?? []).filter((a) => a !== option),
                        }))
                      }
                    />
                    {option === "T" ? "正确" : option === "F" ? "错误" : option}
                  </Label>
                ))}
              </div>
            </div>
          ))}
          <div className="space-y-2">
            <Label htmlFor="regrade-reason">重评原因（记录到历史）</Label>
            <Textarea
              id="regrade-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
          </div>
          <Label className="flex min-h-11 items-start gap-3">
            <Checkbox
              checked={confirmed}
              onCheckedChange={(v) => setConfirmed(v === true)}
            />
            我已核对全部新答案，确认对现有答卷重新计分。
          </Label>
          <DialogFooter>
            <Button
              className="min-h-11"
              variant="outline"
              onClick={() => setOpen(false)}
            >
              取消
            </Button>
            <Button
              className="min-h-11"
              disabled={
                busy ||
                !confirmed ||
                !reason.trim() ||
                exam.questions.some((q) => !answers[q.id!]?.length)
              }
              onClick={() => void submit()}
            >
              {busy ? "正在重评" : "确认重评全部答卷"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
