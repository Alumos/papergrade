import { lazy, Suspense } from "react";
import { useRemote } from "@/hooks";
import { Feedback } from "@/components/app/feedback";
const Markdown = lazy(() => import("./Markdown"));
export default function PageEvidence({
  examID,
  documentID,
  refresh,
}: {
  examID: string;
  documentID: string;
  refresh: number;
}) {
  const pages = useRemote<{
    items: {
      page_number: number;
      status: string;
      text?: string;
      error?: string;
    }[];
  }>(
    "/exams/" +
      examID +
      "/documents/" +
      documentID +
      "/pages?progress=" +
      refresh,
  );
  return (
    <div className="space-y-3">
      {pages.error && <Feedback error>{pages.error}</Feedback>}
      {pages.data?.items
        .filter((p) => p.text || p.error)
        .sort((a, b) => a.page_number - b.page_number)
        .map((p) => (
          <details key={p.page_number}>
            <summary className="cursor-pointer text-sm">
              第{p.page_number}页 OCR · {p.status}
            </summary>
            {p.error && <Feedback error>{p.error}</Feedback>}
            {p.text && (
              <Suspense fallback={<p>正在载入数学排版…</p>}>
                <Markdown text={p.text} />
              </Suspense>
            )}
          </details>
        ))}
    </div>
  );
}
