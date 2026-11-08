import type { Manifest } from "../types";
import type { OMRPreview } from "./omr";
export class OMRWorkerClient {
  private worker: Worker;
  private sequence = 0;
  private pending = new Map<
    number,
    {
      resolve: (r: OMRPreview) => void;
      reject: (e: Error) => void;
      timer: ReturnType<typeof setTimeout>;
    }
  >();
  constructor() {
    this.worker = new Worker(new URL("./omr.worker.ts", import.meta.url), {
      type: "module",
    });
    this.worker.onmessage = (e) => {
      const p = this.pending.get(e.data.id);
      if (!p) return;
      clearTimeout(p.timer);
      this.pending.delete(e.data.id);
      if (e.data.error) p.reject(new Error(e.data.error));
      else p.resolve(e.data.result);
    };
    this.worker.onerror = () => this.close();
  }
  recognize(image: ImageData, manifest: Manifest): Promise<OMRPreview> {
    const id = ++this.sequence;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error("本机OMR超时，请重新取景"));
      }, 20000);
      this.pending.set(id, { resolve, reject, timer });
      this.worker.postMessage(
        {
          id,
          image: { width: image.width, height: image.height, data: image.data },
          manifest,
        },
        [image.data.buffer],
      );
    });
  }
  close() {
    this.worker.terminate();
    for (const p of this.pending.values()) {
      clearTimeout(p.timer);
      p.reject(new Error("OMR已停止"));
    }
    this.pending.clear();
  }
}
