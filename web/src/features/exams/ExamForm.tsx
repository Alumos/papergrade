import { useState } from "react";
import { Plus, Trash2, Save, Send } from "lucide-react";
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
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Feedback } from "@/components/app/feedback";
import { useRemote } from "@/hooks";
import { api, post, errorText } from "@/api";
import { score } from "@/lib";
import type { Exam, Class, Question } from "@/types";
export default function ExamForm({
  exam,
  onSaved,
}: {
  exam?: Exam;
  onSaved: (exam: Exam) => void;
}) {
  const classes = useRemote<{ items: Class[] }>("/classes");
  const [title, setTitle] = useState(exam?.title ?? ""),
    [subject, setSubject] = useState(exam?.subject ?? ""),
    [classID, setClassID] = useState(exam?.class_id ?? ""),
    [questions, setQuestions] = useState<Question[]>(exam?.questions ?? []),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [confirm, setConfirm] = useState(false);
  const change = (index: number, patch: Partial<Question>) =>
    setQuestions((qs) =>
      qs.map((q, i) => (i === index ? { ...q, ...patch } : q)),
    );
  const save = async (publish = false) => {
    setBusy(true);
    setError("");
    try {
      if (!title.trim() || !classID)
        throw new Error("请填写考试名称并选择班级");
      if (
        questions.some(
          (q) =>
            !q.number.trim() ||
            !Number.isInteger(q.score_units) ||
            q.score_units < 0 ||
            (publish && q.score_units === 0),
        )
      )
        throw new Error("请检查题号与分值（草稿可待确认零分；发布须大于零）");
      if (new Set(questions.map((q) => q.number)).size !== questions.length)
        throw new Error("题号不能重复");
      if (
        publish &&
        (!questions.length || questions.some((q) => !q.answer.length))
      )
        throw new Error("发布前每题都需要正确答案");
      const body = {
        title: title.trim(),
        subject: subject.trim(),
        class_id: classID,
        questions,
        revision: exam?.revision,
      };
      let value = exam
        ? await api<Exam>("/exams/" + exam.id, {
            method: "PATCH",
            body: JSON.stringify(body),
          })
        : await post<Exam>("/exams", body);
      if (publish) {
        try {
          value = await post<Exam>("/exams/" + value.id + "/publish", {
            revision: value.revision,
          });
        } catch (e) {
          onSaved(value);
          throw new Error("草稿已保存，但发布失败：" + errorText(e));
        }
      }
      onSaved(value);
      setConfirm(false);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="space-y-6">
      {(error || classes.error) && (
        <Feedback error>{error || classes.error}</Feedback>
      )}
      <Card>
        <CardHeader>
          <CardTitle>考试信息</CardTitle>
          <CardDescription>
            发布后冻结考试与答案，不会覆盖历史版本。
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-5 sm:grid-cols-3">
          <div className="space-y-2">
            <Label htmlFor="title">考试名称</Label>
            <Input
              className="min-h-11"
              id="title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="第一单元测验"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="subject">学科</Label>
            <Input
              className="min-h-11"
              id="subject"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              placeholder="数学"
            />
          </div>
          <div className="space-y-2">
            <Label>班级</Label>
            <Select value={classID} onValueChange={setClassID}>
              <SelectTrigger className="min-h-11 w-full">
                <SelectValue placeholder="选择班级" />
              </SelectTrigger>
              <SelectContent>
                {classes.data?.items.map((c) => (
                  <SelectItem key={c.id} value={c.id}>
                    {c.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between gap-3">
            <CardTitle>客观题配置</CardTitle>
            <Badge variant="outline">
              {questions.length}/40 题 ·{" "}
              {score(questions.reduce((n, q) => n + q.score_units, 0))} 分
            </Badge>
          </div>
          <CardDescription>
            固定单页 A4，每题最多 5 个选项。分数由服务端计算。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {questions.map((q, i) => (
            <Card key={q.id ?? i}>
              <CardContent className="space-y-4 pt-5">
                {!!q.review_flags?.length && (
                  <Feedback>待核对：{q.review_flags.join("；")}</Feedback>
                )}
                <div className="grid grid-cols-[4rem_1fr_5rem_auto] gap-2">
                  <div className="space-y-2">
                    <Label htmlFor={"number-" + i}>题号</Label>
                    <Input
                      className="min-h-11"
                      id={"number-" + i}
                      value={q.number}
                      onChange={(e) => change(i, { number: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>题型</Label>
                    <Select
                      value={q.type}
                      onValueChange={(value) => {
                        const type = value as Question["type"];
                        change(i, {
                          type,
                          options:
                            type === "true_false"
                              ? ["T", "F"]
                              : ["A", "B", "C", "D"],
                          answer: [],
                          partial_score_units: undefined,
                        });
                      }}
                    >
                      <SelectTrigger className="min-h-11 w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="single_choice">单选题</SelectItem>
                        <SelectItem value="multiple_choice">多选题</SelectItem>
                        <SelectItem value="true_false">判断题</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor={"score-" + i}>分值</Label>
                    <Input
                      className="min-h-11"
                      id={"score-" + i}
                      type="number"
                      min="0.01"
                      step="0.01"
                      value={q.score_units / 100}
                      onChange={(e) =>
                        change(i, {
                          score_units: Math.round(Number(e.target.value) * 100),
                        })
                      }
                    />
                  </div>
                  <Button
                    className="min-h-11 self-end"
                    variant="ghost"
                    size="icon"
                    aria-label={"删除第" + q.number + "题"}
                    onClick={() =>
                      setQuestions((qs) => qs.filter((_, j) => j !== i))
                    }
                  >
                    <Trash2 />
                  </Button>
                </div>
                <div className="flex flex-wrap items-center gap-4">
                  <Label>正确答案</Label>
                  {q.options.map((option) => (
                    <Label className="flex items-center gap-2" key={option}>
                      <Checkbox
                        checked={q.answer.includes(option)}
                        onCheckedChange={(checked) =>
                          change(i, {
                            answer: checked
                              ? q.type === "multiple_choice"
                                ? [...q.answer, option]
                                : [option]
                              : q.answer.filter((a) => a !== option),
                          })
                        }
                      />
                      {option === "T"
                        ? "正确"
                        : option === "F"
                          ? "错误"
                          : option}
                    </Label>
                  ))}
                  {q.type !== "true_false" && (
                    <Select
                      value={String(q.options.length)}
                      onValueChange={(v) => {
                        const options = ["A", "B", "C", "D", "E"].slice(
                          0,
                          Number(v),
                        );
                        change(i, {
                          options,
                          answer: q.answer.filter((a) => options.includes(a)),
                        });
                      }}
                    >
                      <SelectTrigger
                        className="min-h-11 ml-auto w-28"
                        aria-label="选项数量"
                      >
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {[2, 3, 4, 5].map((n) => (
                          <SelectItem key={n} value={String(n)}>
                            {n} 个选项
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                </div>
                {q.type === "multiple_choice" && (
                  <div className="space-y-2 max-w-56">
                    <Label htmlFor={"partial-" + i}>少选且无错选得分</Label>
                    <Input
                      className="min-h-11"
                      id={"partial-" + i}
                      type="number"
                      min="0"
                      step="0.01"
                      value={(q.partial_score_units ?? 0) / 100}
                      onChange={(e) =>
                        change(i, {
                          partial_score_units: Math.round(
                            Number(e.target.value) * 100,
                          ),
                        })
                      }
                    />
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </CardContent>
        <CardFooter>
          <Button
            className="min-h-11"
            variant="outline"
            disabled={questions.length >= 40}
            onClick={() =>
              setQuestions((qs) => [
                ...qs,
                {
                  number: String(qs.length + 1),
                  type: "single_choice",
                  options: ["A", "B", "C", "D"],
                  answer: [],
                  score_units: 200,
                },
              ])
            }
          >
            <Plus />
            添加题目
          </Button>
        </CardFooter>
      </Card>
      <div className="sticky bottom-3 flex justify-end gap-2 rounded-lg border bg-background p-3">
        <Button
          className="min-h-11"
          variant="outline"
          disabled={busy}
          onClick={() => void save()}
        >
          <Save />
          {busy ? "保存中" : "保存草稿"}
        </Button>
        <Button
          className="min-h-11"
          disabled={busy || !questions.length}
          onClick={() => setConfirm(true)}
        >
          <Send />
          发布考试
        </Button>
      </div>
      <Dialog open={confirm} onOpenChange={setConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认发布考试？</DialogTitle>
            <DialogDescription>
              将冻结答案及个人卡版本。发布后不可直接编辑，请再次核对所有标准答案。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              className="min-h-11"
              variant="outline"
              onClick={() => setConfirm(false)}
            >
              取消
            </Button>
            <Button
              className="min-h-11"
              disabled={busy}
              onClick={() => void save(true)}
            >
              保存并发布
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
