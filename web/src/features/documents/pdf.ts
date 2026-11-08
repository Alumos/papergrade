import { api } from "@/api";
export const MAX_PDF_PAGES = 30;
export function fitPage(width: number, height: number) {
  const scale = Math.min(1, 2048 / Math.max(width, height));
  return {
    width: Math.round(width * scale),
    height: Math.round(height * scale),
    scale,
  };
}
export async function preparePDF(
  examID: string,
  documentID: string,
  file: File,
  onProgress: (page: number, total: number) => void,
  signal?: AbortSignal,
  preparedPages = new Set<number>(),
) {
  const pdfjs = await import("pdfjs-dist");
  pdfjs.GlobalWorkerOptions.workerSrc = new URL(
    "pdfjs-dist/build/pdf.worker.min.mjs",
    import.meta.url,
  ).href;
  const loading = pdfjs.getDocument({
    data: new Uint8Array(await file.arrayBuffer()),
    useSystemFonts: true,
  });
  const pdf = await loading.promise;
  try {
    if (pdf.numPages > MAX_PDF_PAGES)
      throw new Error("PDF最多30页，请拆分后上传");
    for (let n = 1; n <= pdf.numPages; n++) {
      if (preparedPages.has(n)) {
        onProgress(n, pdf.numPages);
        continue;
      }
      if (signal?.aborted)
        throw new DOMException("已停止页面准备", "AbortError");
      const page = await pdf.getPage(n);
      const base = page.getViewport({ scale: 1 });
      const scale = 2048 / Math.max(base.width, base.height);
      const viewport = page.getViewport({ scale });
      const canvas = document.createElement("canvas");
      canvas.width = Math.ceil(viewport.width);
      canvas.height = Math.ceil(viewport.height);
      try {
        await page.render({ canvas, viewport }).promise;
        const blob = await new Promise<Blob>((resolve, reject) =>
          canvas.toBlob(
            (b) => (b ? resolve(b) : reject(new Error("页面JPEG编码失败"))),
            "image/jpeg",
            0.88,
          ),
        );
        if (blob.size > 4 * 1024 * 1024)
          throw new Error("第" + n + "页超过4MB，请降低源文件复杂度");
        const form = new FormData();
        form.set("image", blob, "page-" + n + ".jpg");
        form.set("page_number", String(n));
        form.set("total_pages", String(pdf.numPages));
        await api("/exams/" + examID + "/documents/" + documentID + "/pages", {
          method: "POST",
          body: form,
          signal,
        });
        onProgress(n, pdf.numPages);
      } finally {
        canvas.width = 0;
        canvas.height = 0;
        page.cleanup();
      }
    }
  } finally {
    await loading.destroy();
  }
}
