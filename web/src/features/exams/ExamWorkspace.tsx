import { lazy, Suspense, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { PageTitle, Loading, Feedback } from "@/components/app/feedback";
import { useRemote } from "@/hooks";
import { score } from "@/lib";
import type { Exam } from "@/types";
import ExamForm from "./ExamForm";
import PrintPanel from "@/features/print/PrintPanel";
import ResultsPage from "@/features/results/ResultsPage";
import DocumentsPage from "@/features/documents/DocumentsPage";
const ScanPage = lazy(() => import("@/features/scan/ScanPage"));
export default function ExamWorkspace({
  id,
  onBack,
}: {
  id: string;
  onBack: () => void;
}) {
  const exam = useRemote<Exam>("/exams/" + id);
  const [tab, setTab] = useState("setup");
  if (exam.loading) return <Loading />;
  if (exam.error)
    return (
      <Feedback error>
        {exam.error}
        <Button className="min-h-11" variant="link" onClick={exam.reload}>
          重试
        </Button>
      </Feedback>
    );
  if (!exam.data) return null;
  const value = exam.data;
  return (
    <div className="space-y-6">
      <Button className="min-h-11" variant="ghost" onClick={onBack}>
        <ArrowLeft />
        返回考试
      </Button>
      <PageTitle
        title={value.title}
        description={
          (value.subject || "客观题测验") +
          " · " +
          value.questions.length +
          " 题 · " +
          score(value.questions.reduce((n, q) => n + q.score_units, 0)) +
          " 分"
        }
        action={
          <Badge variant="secondary">
            {value.status === "draft" ? "草稿 · v" + value.revision : "已发布"}
          </Badge>
        }
      />
      <Tabs value={tab} onValueChange={setTab} className="space-y-5">
        <div className="overflow-x-auto pb-1">
          <TabsList className="min-w-max">
            <TabsTrigger className="min-h-11" value="setup">
              {value.status === "draft" ? "配置草稿" : "个人卡与打印"}
            </TabsTrigger>
            <TabsTrigger
              className="min-h-11"
              value="scan"
              disabled={value.status === "draft"}
            >
              连续扫描
            </TabsTrigger>
            <TabsTrigger
              className="min-h-11"
              value="results"
              disabled={value.status === "draft"}
            >
              结果与复核
            </TabsTrigger>
            <TabsTrigger className="min-h-11" value="documents">
              PDF 与解析
            </TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value="setup">
          {value.status === "draft" ? (
            <ExamForm
              key={value.id + value.revision}
              exam={value}
              onSaved={exam.setData}
            />
          ) : (
            <PrintPanel exam={value} />
          )}
        </TabsContent>
        <TabsContent value="scan">
          <Suspense fallback={<Loading />}>
            <ScanPage exam={value} />
          </Suspense>
        </TabsContent>
        <TabsContent value="results">
          <ResultsPage exam={value} onChanged={exam.setData} />
        </TabsContent>
        <TabsContent value="documents">
          <DocumentsPage exam={value} onChanged={exam.setData} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
