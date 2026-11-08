export async function fetchPrintSheet(examID: string, studentIDs: string[]) {
  const query = studentIDs.map(encodeURIComponent).join(",");
  const response = await fetch(
    "/api/v1/exams/" +
      encodeURIComponent(examID) +
      "/print-sheet?student_ids=" +
      query,
    { credentials: "same-origin" },
  );
  if (!response.ok)
    throw new Error("双联答题卡加载失败 (" + response.status + ")");
  return await response.text();
}
export async function printTwoUp(examID: string, studentIDs: string[]) {
  if (studentIDs.length < 1 || studentIDs.length > 2)
    throw new Error("一次只能打印1或2份双联答题卡");
  const svg = await fetchPrintSheet(examID, studentIDs);
  const popup = window.open("about:blank", "_blank");
  if (!popup) throw new Error("请允许弹出窗口以打印双联答题卡");
  try {
    popup.document.title = "A4 双联答题卡";
    popup.document.body.innerHTML = svg;
    const style = popup.document.createElement("style");
    style.textContent =
      "@page{size:A4;margin:0}html,body{margin:0;width:210mm;height:297mm}svg{display:block;width:210mm;height:297mm}";
    popup.document.head.append(style);
    popup.focus();
    popup.print();
  } catch (error) {
    popup.close();
    throw error;
  }
}
export async function downloadTwoUpSVG(examID: string, studentIDs: string[]) {
  const svg = await fetchPrintSheet(examID, studentIDs);
  const blob = new Blob([svg], { type: "image/svg+xml;charset=utf-8" });
  const href = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = href;
  anchor.download = "papergrade-two-up.svg";
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(href), 1000);
}
export async function downloadTwoUpPDF(examID: string, studentIDs: string[]) {
  const svgText = await fetchPrintSheet(examID, studentIDs);
  // Render only the authenticated fixed SVG, not the application UI. Browser font
  // rendering preserves CJK labels; embedding that exact 300dpi page avoids jsPDF
  // built-in Latin-only font substitution. Geometry remains 210×297mm.
  const image = new Image();
  image.src = "data:image/svg+xml;charset=utf-8," + encodeURIComponent(svgText);
  await image.decode();
  const canvas = document.createElement("canvas");
  canvas.width = 2480;
  canvas.height = 3508;
  canvas.getContext("2d")!.fillStyle = "white";
  canvas.getContext("2d")!.fillRect(0, 0, canvas.width, canvas.height);
  canvas.getContext("2d")!.drawImage(image, 0, 0, canvas.width, canvas.height);
  const { jsPDF } = await import("jspdf");
  const pdf = new jsPDF({ orientation: "portrait", unit: "mm", format: "a4" });
  pdf.addImage(canvas.toDataURL("image/jpeg", 0.96), "JPEG", 0, 0, 210, 297);
  canvas.width = 0;
  canvas.height = 0;
  pdf.save("papergrade-two-up.pdf");
}
