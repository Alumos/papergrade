import { useState } from "react";
import { Plus, ArrowRight, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  PageTitle,
  Feedback,
  Loading,
  Empty,
  Status,
} from "@/components/app/feedback";
import { useRemote } from "@/hooks";
import type { Exam, Class } from "@/types";
export default function ExamsPage({
  onCreate,
  onSelect,
}: {
  onCreate: () => void;
  onSelect: (id: string) => void;
}) {
  const exams = useRemote<{ items: Exam[] }>("/exams"),
    classes = useRemote<{ items: Class[] }>("/classes");
  const [filter, setFilter] = useState("all");
  const all = exams.data?.items ?? [],
    list = all.filter((e) => filter === "all" || e.status === filter);
  return (
    <div className="space-y-6">
      <PageTitle
        title="考试工作台"
        description="从配置个人卡，到扫描、统计与复核。"
        action={
          <Button className="min-h-11" onClick={onCreate}>
            <Plus />
            新建考试
          </Button>
        }
      />
      {(exams.error || classes.error) && (
        <Feedback error>{exams.error || classes.error}</Feedback>
      )}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        {[
          ["已发布考试", all.filter((e) => e.status === "published").length],
          ["班级", classes.data?.items.length ?? 0],
          ["已收答卷", all.reduce((n, e) => n + (e.received_count ?? 0), 0)],
          ["待复核", all.reduce((n, e) => n + (e.review_count ?? 0), 0)],
        ].map(([label, value]) => (
          <Card key={label}>
            <CardHeader>
              <CardDescription>{label}</CardDescription>
              <CardTitle className="text-3xl tabular-nums">
                {exams.loading ? "—" : value}
              </CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>
      <div className="flex justify-between items-center">
        <Tabs value={filter} onValueChange={setFilter}>
          <TabsList>
            <TabsTrigger className="min-h-11" value="all">
              全部考试
            </TabsTrigger>
            <TabsTrigger className="min-h-11" value="published">
              已发布
            </TabsTrigger>
            <TabsTrigger className="min-h-11" value="draft">
              草稿
            </TabsTrigger>
          </TabsList>
        </Tabs>
        <Button
          className="min-h-11"
          variant="outline"
          size="icon"
          aria-label="刷新考试"
          onClick={exams.reload}
        >
          <RefreshCw />
        </Button>
      </div>
      {exams.loading ? (
        <Loading />
      ) : !list.length ? (
        <Empty>还没有考试。先建立班级，再创建第一场考试。</Empty>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {list.map((exam) => (
            <Card key={exam.id}>
              <CardHeader>
                <div className="flex justify-between items-center">
                  <Badge variant="outline">
                    {exam.subject || "客观题测验"}
                  </Badge>
                  <Status value={exam.status} />
                </div>
                <CardTitle className="pt-3">{exam.title}</CardTitle>
                <CardDescription>
                  {classes.data?.items.find((c) => c.id === exam.class_id)
                    ?.name ?? "关联班级"}{" "}
                  · {exam.questions.length} 题
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">已收答卷</span>
                  <span>
                    {exam.received_count ?? 0} / {exam.student_count ?? 0}
                  </span>
                </div>
                {exam.review_count > 0 && (
                  <p className="mt-2 text-sm text-muted-foreground">
                    {exam.review_count} 份待复核
                  </p>
                )}
              </CardContent>
              <CardFooter className="justify-between">
                <span className="text-xs text-muted-foreground">
                  {new Date(exam.created_at).toLocaleDateString("zh-CN")}
                </span>
                <Button
                  className="min-h-11"
                  variant="ghost"
                  onClick={() => onSelect(exam.id)}
                >
                  {exam.status === "draft" ? "编辑草稿" : "进入考试"}
                  <ArrowRight />
                </Button>
              </CardFooter>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
