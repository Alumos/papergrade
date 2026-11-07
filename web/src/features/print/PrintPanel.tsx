import { useState } from "react";
import { Printer } from "lucide-react";
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
  const [selected, setSelected] = useState<string[]>([]),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const print = async (ids: string[]) => {
    setBusy(true);
    setError("");
    try {
      await printCards(exam.id, ids);
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
        打印，不要缩放或裁去四角。每人使用独立二维码。可在浏览器打印窗口另存
        PDF。
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
                选择需要打印的学生，卡片由服务器生成。
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
