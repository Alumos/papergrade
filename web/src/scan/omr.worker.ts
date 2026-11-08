import { recognizeOMR } from "./omr";
self.onmessage = (event: MessageEvent) => {
  const { id, image, manifest } = event.data;
  try {
    const result = recognizeOMR(image, manifest);
    self.postMessage({ id, result });
  } catch (error) {
    self.postMessage({
      id,
      error: error instanceof Error ? error.message : "浏览器OMR失败",
    });
  }
};
