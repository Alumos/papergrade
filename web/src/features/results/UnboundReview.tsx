import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Feedback } from "@/components/app/feedback";
import { useRemote } from "@/hooks";
import { post, errorText } from "@/api";
import type { CaptureResult, Exam, Sheet } from "@/types";
export default function UnboundReview({
  exam,
  onBound,
}: {
  exam: Exam;
  onBound: () => void;
}) {
  const captures = useRemote<{ items: CaptureResult[] }>(
      "/exams/" + exam.id + "/unbound-captures",
    ),
    sheets = useRemote<{ items: Sheet[] }>("/exams/" + exam.id + "/sheets");
  const [selected, setSelected] = useState<CaptureResult | null>(null),
    [student, setStudent] = useState(""),
    [reason, setReason] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const bind = async () => {
    if (!selected) return;
    setBusy(true);
    setError("");
    try {
      await post("/captures/" + selected.capture_id + "/bind", {
        revision: selected.revision,
        student_id: student,
        reason: reason.trim(),
      });
      setSelected(null);
      captures.reload();
      onBound();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  if (!captures.data?.items.length && !captures.error) return null;
  return (
    <Card>
      <CardHeader>
        <CardTitle>学号未绑定 · 身份待复核</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {captures.error && <Feedback error>{captures.error}</Feedback>}
        {captures.data?.items.map((c) => (
          <div
            key={c.capture_id}
            className="flex flex-wrap justify-between items-center gap-3 border-b pb-3"
          >
            <p className="text-sm">
              学号 {c.student_number || "未确定"} ·{" "}
              {c.identity_state ?? "uncertain"} · {c.message}
            </p>
            <Button
              className="min-h-11"
              variant="outline"
              onClick={() => {
                setSelected(c);
                setStudent("");
                setReason("");
                setError("");
              }}
            >
              查看原图并绑定
            </Button>
          </div>
        ))}
        <Dialog open={!!selected} onOpenChange={(v) => !v && setSelected(null)}>
          <DialogContent className="max-h-[90svh] overflow-auto">
            <DialogHeader>
              <DialogTitle>人工确认学号归属</DialogTitle>
              <DialogDescription>
                试卷二维码不代表学生。请对照原图学号/手写区明确绑定，服务端按识别答案计分且不会覆盖已有答卷。
              </DialogDescription>
            </DialogHeader>
            {selected && (
              <img
                src={"/api/v1/captures/" + selected.capture_id + "/image"}
                alt="未绑定答卷原图"
                className="max-h-72 w-full object-contain border"
              />
            )}
            <div className="space-y-2">
              <Label>名册学生（保留前导零）</Label>
              <Select value={student} onValueChange={setStudent}>
                <SelectTrigger className="min-h-11 w-full">
                  <SelectValue placeholder="明确选择学生" />
                </SelectTrigger>
                <SelectContent>
                  {sheets.data?.items.map((s) => (
                    <SelectItem key={s.student_id} value={s.student_id}>
                      {s.student_number} · {s.student_name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Label htmlFor="bind-reason">绑定依据</Label>
            <Textarea
              id="bind-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
            {error && <Feedback error>{error}</Feedback>}
            <DialogFooter>
              <Button
                className="min-h-11"
                variant="outline"
                onClick={() => setSelected(null)}
              >
                取消
              </Button>
              <Button
                className="min-h-11"
                disabled={busy || !student || !reason.trim()}
                onClick={() => void bind()}
              >
                确认绑定并由服务端计分
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardContent>
    </Card>
  );
}
