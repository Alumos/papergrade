import { useState } from "react";
import { Plus, Upload } from "lucide-react";
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
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Table,
  TableHeader,
  TableRow,
  TableHead,
  TableBody,
  TableCell,
} from "@/components/ui/table";
import { PageTitle, Feedback, Loading, Empty } from "@/components/app/feedback";
import { useRemote } from "@/hooks";
import { post, errorText } from "@/api";
import { parseRoster } from "@/lib";
import type { Class, Student } from "@/types";
export default function ClassesPage() {
  const classes = useRemote<{ items: Class[] }>("/classes");
  const [id, setID] = useState(""),
    [name, setName] = useState(""),
    [csv, setCSV] = useState(""),
    [open, setOpen] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [message, setMessage] = useState("");
  const students = useRemote<{ items: Student[] }>(
    id ? "/classes/" + id + "/students" : null,
  );
  const create = async () => {
    setBusy(true);
    setError("");
    try {
      const result = await post<Class>("/classes", { name: name.trim() });
      setID(result.id);
      setOpen(false);
      setName("");
      classes.reload();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  const upload = async () => {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const entries = parseRoster(csv);
      if (!entries.length) throw new Error("请粘贴名单");
      const result = await post<{ imported: number }>(
        "/classes/" + id + "/students",
        { students: entries },
      );
      setMessage("已保存 " + result.imported + " 位学生");
      setCSV("");
      students.reload();
      classes.reload();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="space-y-6">
      <PageTitle
        title="班级与名册"
        description="保留学号前导零，重复学号整批拒绝。"
        action={
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button className="min-h-11">
                <Plus />
                新建班级
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>新建班级</DialogTitle>
                <DialogDescription>
                  为日常考试建立一个学生分组。
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-2">
                <Label htmlFor="class-name">班级名称</Label>
                <Input
                  className="min-h-11"
                  id="class-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="七年级一班"
                />
              </div>
              <DialogFooter>
                <Button
                  className="min-h-11"
                  disabled={busy || !name.trim()}
                  onClick={() => void create()}
                >
                  创建
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        }
      />
      {(error || classes.error || students.error) && (
        <Feedback error>{error || classes.error || students.error}</Feedback>
      )}
      {message && <Feedback>{message}</Feedback>}
      <div className="grid gap-6 md:grid-cols-[.85fr_1.15fr]">
        <Card>
          <CardHeader>
            <CardTitle>导入名单</CardTitle>
            <CardDescription>
              两列 CSV / TSV：学号、姓名。支持表头及引号。
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="space-y-2">
              <Label>选择班级</Label>
              <Select value={id} onValueChange={setID}>
                <SelectTrigger className="min-h-11">
                  <SelectValue placeholder="请选择班级" />
                </SelectTrigger>
                <SelectContent>
                  {classes.data?.items.map((c) => (
                    <SelectItem key={c.id} value={c.id}>
                      {c.name} · {c.student_count} 人
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="csv">CSV 文本</Label>
              <Textarea
                id="csv"
                rows={8}
                value={csv}
                onChange={(e) => setCSV(e.target.value)}
                placeholder={"学号,姓名\n001,张同学\n002,李同学"}
              />
            </div>
            <Button
              className="min-h-11"
              disabled={busy || !id || !csv.trim()}
              onClick={() => void upload()}
            >
              <Upload />
              {busy ? "正在保存" : "验证并导入"}
            </Button>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>学生名单</CardTitle>
            <CardDescription>
              {classes.data?.items.find((c) => c.id === id)?.name ??
                "选择班级后显示"}{" "}
              · {students.data?.items.length ?? 0} 人
            </CardDescription>
          </CardHeader>
          <CardContent>
            {students.loading ? (
              <Loading />
            ) : students.data?.items.length ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>学号</TableHead>
                    <TableHead>姓名</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {students.data.items.map((s) => (
                    <TableRow key={s.id}>
                      <TableCell className="font-mono">{s.number}</TableCell>
                      <TableCell>{s.name}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <Empty>暂无学生。请先选择班级或导入名单。</Empty>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
