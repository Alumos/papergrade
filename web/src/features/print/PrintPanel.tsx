import { useState } from "react";
import { Printer, Download } from "lucide-react";
import {
  fetchPrintSheet,
  downloadTwoUpPDF,
  downloadTwoUpSVG,
} from "./printSheet";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Table,
  TableHeader,
  TableRow,
  TableHead,
  TableBody,
  TableCell,
} from "@/components/ui/table";
import { Feedback, Loading, Empty } from "@/components/app/feedback";
import { useRemote } from "@/hooks";
import { printCards, errorText } from "@/api";
import type { Exam, Sheet, Manifest } from "@/types";
export default function PrintPanel({ exam }: { exam: Exam }) {
  const sheets = useRemote<{ items: Sheet[] }>("/exams/" + exam.id + "/sheets"),
    manifest = useRemote<Manifest>("/exams/" + exam.id + "/template");
  // Both fixed half-A4 cards print two-up and require cutting before scanning.
  // The English card (a5-v3) uses the same rule, so never test for one version.
  const twoUp =
    manifest.data?.version === "a5-v2" || manifest.data?.version === "a5-v3";
  const english = manifest.data?.version === "a5-v3";
  const digits =
    exam.student_number_digits ?? manifest.data?.student_number_digits ?? "";
  const [selected, setSelected] = useState<string[]>([]),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const print = async (ids: string[]) => {
    setBusy(true);
    setError("");
    try {
      if (!twoUp) {
        await printCards(exam.id, ids);
        return;
      }
      const popup = window.open("about:blank", "_blank");
      if (!popup) throw new Error("请允许打印窗口");
      popup.document.body.textContent = "正在准备A4双联答题卡…";
      try {
        const sources: string[] = [];
        for (let i = 0; i < ids.length; i += 2)
          sources.push(await fetchPrintSheet(exam.id, ids.slice(i, i + 2)));
        popup.document.body.textContent = "";
        const style = popup.document.createElement("style");
        style.textContent =
          "@page{size:A4;margin:0}body{margin:0}img{width:210mm;height:297mm;display:block;break-after:page}";
        popup.document.head.append(style);
        await Promise.all(
          sources.map(
            (svg) =>
              new Promise<void>((resolve, reject) => {
                const img = popup.document.createElement("img");
                img.onload = () => resolve();
                img.onerror = () => reject(new Error("模板无法显示"));
                img.src =
                  "data:image/svg+xml;charset=utf-8," + encodeURIComponent(svg);
                popup.document.body.append(img);
              }),
          ),
        );
        popup.focus();
        popup.print();
      } catch (e) {
        popup.close();
        throw e;
      }
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="space-y-5">
      <Feedback>
        模板 {manifest.data?.version ?? "加载中"} · A4 原尺寸 100%
        打印，不要缩放或裁去四角。
        {twoUp
          ? "新卡A4上下两份，沿中线裁切后每次只扫一张210×148.5mm卡。试卷二维码全班相同，仅匹配试卷，不作学生身份。"
          : "旧版每人独立二维码。"}
        可在打印窗口另存PDF。
      </Feedback>
      {(error || sheets.error || manifest.error) && (
        <Feedback error>{error || sheets.error || manifest.error}</Feedback>
      )}
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="space-y-2">
              <CardTitle>个人答题卡</CardTitle>
              <CardDescription>
                选择学生，卡片由服务端固定版式生成。
                {twoUp
                  ? english
                    ? "英语卡：学号固定填涂4位数字（例如19班32号填1932），每行只能涂一个数字。选项框里印着字母，2B涂满整个方框即可；七选五为A–G。"
                    : "学号必须按 " +
                      digits +
                      " 位逐列填涂，前导零不可漏填。2B涂满，每列只选一位；姓名/学号手写区不代替填涂。"
                  : ""}
              </CardDescription>
            </div>
            <Button
              className="min-h-11"
              disabled={busy || !selected.length}
              onClick={() => void print(selected)}
            >
              <Printer />
              {busy ? "加载中" : "打印所选 (" + selected.length + ")"}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {twoUp && (
            <div className="mb-5 space-y-3">
              <p className="text-sm">
                已选 {selected.length} 人 · A4 {Math.ceil(selected.length / 2)}{" "}
                页（末页奇数时下半留空）
              </p>
              <div className="flex flex-wrap gap-2">
                <Button
                  className="min-h-11"
                  variant="outline"
                  disabled={
                    busy || selected.length === 0 || selected.length > 2
                  }
                  onClick={async () => {
                    setBusy(true);
                    try {
                      await downloadTwoUpSVG(exam.id, selected);
                    } catch (e) {
                      setError(errorText(e));
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  <Download />
                  下载双联SVG（最多2人）
                </Button>
                <Button
                  className="min-h-11"
                  variant="outline"
                  disabled={
                    busy || selected.length === 0 || selected.length > 2
                  }
                  onClick={async () => {
                    setBusy(true);
                    try {
                      await downloadTwoUpPDF(exam.id, selected);
                    } catch (e) {
                      setError(errorText(e));
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  <Download />
                  下载真实PDF（最多2人）
                </Button>
              </div>
              <div className="mx-auto w-48 border bg-white text-center text-xs text-muted-foreground">
                <div className="h-32 grid place-content-center">
                  上联 · 210×148.5mm
                  <br />
                  学号填涂 + 客观题
                </div>
                <div className="border-t border-dashed py-1">裁切线</div>
                <div className="h-32 grid place-content-center">
                  {selected.length % 2 === 1
                    ? "末页下联留白"
                    : "下联 · 210×148.5mm"}
                </div>
              </div>
            </div>
          )}
          {sheets.loading ? (
            <Loading />
          ) : !sheets.data?.items.length ? (
            <Empty>没有可打印的个人卡，请检查发布时的班级名单。</Empty>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10">
                    <Checkbox
                      aria-label="全选学生"
                      checked={selected.length === sheets.data.items.length}
                      onCheckedChange={(v) =>
                        setSelected(
                          v ? sheets.data!.items.map((s) => s.student_id) : [],
                        )
                      }
                    />
                  </TableHead>
                  <TableHead>学号</TableHead>
                  <TableHead>姓名</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sheets.data.items.map((s) => (
                  <TableRow key={s.student_id}>
                    <TableCell>
                      <Checkbox
                        aria-label={"选择" + s.student_name}
                        checked={selected.includes(s.student_id)}
                        onCheckedChange={(v) =>
                          setSelected((ids) =>
                            v
                              ? [...ids, s.student_id]
                              : ids.filter((id) => id !== s.student_id),
                          )
                        }
                      />
                    </TableCell>
                    <TableCell className="font-mono">
                      {s.student_number}
                    </TableCell>
                    <TableCell>{s.student_name}</TableCell>
                    <TableCell className="text-right">
                      <Button
                        className="min-h-11"
                        variant="ghost"
                        disabled={busy}
                        onClick={() => void print([s.student_id])}
                      >
                        单独打印
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
        发布后的试卷与答案已冻结。固定一页客观题标准卡，不支持任意试卷版式。
      </Feedback>
    </div>
  );
}
