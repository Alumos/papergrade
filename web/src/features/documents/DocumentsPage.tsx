import { useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Checkbox } from "@/components/ui/checkbox";
import { Upload, RefreshCw, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
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
import { Separator } from "@/components/ui/separator";
import { Feedback, Status, Empty } from "@/components/app/feedback";
import { useRemote } from "@/hooks";
import { api, post, errorText } from "@/api";
import type { DocumentItem, Exam } from "@/types";
export default function DocumentsPage({
  exam,
  onChanged,
}: {
  exam: Exam;
  onChanged: (exam: Exam) => void;
}) {
  const capabilities = useRemote<{ document_adopt_available: boolean }>(
    "/settings/capabilities",
  );
  const [adoptID, setAdoptID] = useState(""),
    [replace, setReplace] = useState(false);
  const documents = useRemote<{ items: DocumentItem[] }>(
    "/exams/" + exam.id + "/documents",
  );
  const [file, setFile] = useState<File | null>(null),
    [id, setID] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [message, setMessage] = useState("");
  useEffect(() => {
    if (
      !documents.data?.items.some((d) =>
        ["queued", "processing"].includes(d.status),
      )
    )
      return;
    const timer = setTimeout(documents.reload, 4000);
    return () => clearTimeout(timer);
  }, [documents.data, documents.reload]);
  const upload = async () => {
    if (!file) return;
    setBusy(true);
    setError("");
    try {
      if (!file.name.toLowerCase().endsWith(".pdf"))
        throw new Error("请选择 PDF");
      if (file.size > 20 * 1024 * 1024) throw new Error("PDF 不能超过20MB");
      const form = new FormData();
      form.set("file", file);
      const result = await api<DocumentItem>(
        "/exams/" + exam.id + "/documents",
        { method: "POST", body: form },
      );
      setID(result.id);
      setMessage("PDF 已上传，尚不代表已解析。");
      documents.reload();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  const parse = async () => {
    setBusy(true);
    setError("");
    try {
      const result = await post<DocumentItem>("/exams/" + exam.id + "/parse", {
        document_id: id,
      });
      setMessage(
        "服务器处理状态：" + result.status + "。提取候选必须由教师确认。",
      );
      documents.reload();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  const adopt = async () => {
    setBusy(true);
    setError("");
    try {
      const result = await post<{
        exam: Exam;
        imported: number;
        skipped: { number: string; reason: string }[];
        warnings: string[];
      }>("/exams/" + exam.id + "/documents/" + adoptID + "/adopt", {
        revision: exam.revision,
        replace_existing: replace,
      });
      onChanged(result.exam);
      setAdoptID("");
      setMessage(
        "已采纳 " +
          result.imported +
          " 题到草稿。" +
          result.skipped.map((s) => s.number + "：" + s.reason).join("；") +
          result.warnings.join("；"),
      );
      documents.reload();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="space-y-5">
      <Dialog open={!!adoptID} onOpenChange={(v) => !v && setAdoptID("")}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>采纳候选到草稿</DialogTitle>
            <DialogDescription>
              只采纳结构合法的客观题。不完整答案/分值保留为待确认，必须核对后才能发布。
            </DialogDescription>
          </DialogHeader>
          {exam.questions.length > 0 && (
            <Label className="flex min-h-11 items-start gap-3">
              <Checkbox
                checked={replace}
                onCheckedChange={(v) => setReplace(v === true)}
              />
              当前草稿已有 {exam.questions.length}{" "}
              题。我明确同意用候选替换，原草稿不会被静默覆盖。
            </Label>
          )}
          {error && <Feedback error>{error}</Feedback>}
          <DialogFooter>
            <Button
              className="min-h-11"
              variant="outline"
              onClick={() => setAdoptID("")}
            >
              取消
            </Button>
            <Button
              className="min-h-11"
              disabled={busy || (exam.questions.length > 0 && !replace)}
              onClick={() => void adopt()}
            >
              确认采纳到草稿
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      {(error || documents.error) && (
        <Feedback error>{error || documents.error}</Feedback>
      )}
      {message && <Feedback>{message}</Feedback>}
      <Card>
        <CardHeader>
          <CardTitle>PDF 上传与云端解析</CardTitle>
          <CardDescription>
            原卷保存在服务端，未配置供应商时显示真实错误。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="space-y-2">
            <Label htmlFor="pdf">试卷 PDF（≤20MB）</Label>
            <Input
              className="min-h-11"
              id="pdf"
              type="file"
              accept="application/pdf,.pdf"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
          </div>
          <Button
            className="min-h-11"
            disabled={busy || !file}
            onClick={() => void upload()}
          >
            <Upload />
            上传 PDF
          </Button>
          <Separator />
          <div className="flex flex-wrap gap-2">
            <Select value={id} onValueChange={setID}>
              <SelectTrigger className="min-h-11 w-full sm:w-64">
                <SelectValue placeholder="选择已上传文档" />
              </SelectTrigger>
              <SelectContent>
                {documents.data?.items.map((d) => (
                  <SelectItem key={d.id} value={d.id}>
                    {d.filename}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              className="min-h-11"
              disabled={busy || !id}
              onClick={() => void parse()}
            >
              <Sparkles />
              请求云端解析
            </Button>
            <Button
              className="min-h-11"
              variant="outline"
              onClick={documents.reload}
            >
              <RefreshCw />
              刷新
            </Button>
          </div>
        </CardContent>
      </Card>
      {!documents.data?.items.length ? (
        <Empty>尚无上传文档。</Empty>
      ) : (
        documents.data.items.map((doc) => (
          <Card key={doc.id}>
            <CardHeader>
              <div className="flex justify-between gap-3">
                <CardTitle className="break-all">{doc.filename}</CardTitle>
                <Status value={doc.status} />
              </div>
              <CardDescription>
                <a
                  href={"/api/v1/documents/" + doc.id + "/file"}
                  target="_blank"
                  rel="noreferrer"
                  className="underline"
                >
                  查看原始 PDF ↗
                </a>
              </CardDescription>
            </CardHeader>
            <CardContent>
              {doc.artifact?.error && (
                <Feedback error>{doc.artifact.error}</Feedback>
              )}
              {doc.artifact?.analysis &&
                capabilities.data?.document_adopt_available &&
                exam.status === "draft" && (
                  <Button
                    className="mb-4 min-h-11"
                    variant="outline"
                    onClick={() => {
                      setReplace(false);
                      setError("");
                      setAdoptID(doc.id);
                    }}
                  >
                    审阅并采纳到草稿
                  </Button>
                )}
              {doc.artifact?.analysis && (
                <details>
                  <summary className="cursor-pointer text-sm font-medium">
                    查看提取候选（未写入草稿） ·{" "}
                    {doc.artifact.analysis.questions.length} 题
                  </summary>
                  <div className="space-y-4 pt-4">
                    <Feedback>
                      模型 {doc.artifact.analysis.model} ·
                      请核对后显式采纳到草稿，再逐题确认；也可人工录入。
                    </Feedback>
                    {doc.artifact.analysis.warnings?.map((w, i) => (
                      <p className="text-sm text-muted-foreground" key={i}>
                        {w}
                      </p>
                    ))}
                    {doc.artifact.analysis.questions.map((q, i) => (
                      <div className="border-b pb-3 text-sm" key={i}>
                        <p className="font-medium">
                          第 {q.number} 题 · {q.type}
                        </p>
                        <p className="text-muted-foreground">
                          选项 {q.options.join("、")} · 答案{" "}
                          {q.answer.join("、") || "待确认"} · 分值{" "}
                          {q.score_units === null
                            ? "待确认"
                            : q.score_units / 100}
                        </p>
                        <p className="text-muted-foreground">
                          {q.review_flags?.join("；")}
                        </p>
                      </div>
                    ))}
                    {doc.artifact.extraction?.text && (
                      <details>
                        <summary className="text-sm cursor-pointer">
                          原始 OCR 文本
                        </summary>
                        <pre className="max-h-72 overflow-auto whitespace-pre-wrap text-xs pt-3">
                          {doc.artifact.extraction.text}
                        </pre>
                      </details>
                    )}
                  </div>
                </details>
              )}
            </CardContent>
          </Card>
        ))
      )}
      <Feedback>
        上传和解析不是发布考试。OCR / AI
        未配置不会产生假题目；可继续手工建立客观题。
      </Feedback>
    </div>
  );
}
