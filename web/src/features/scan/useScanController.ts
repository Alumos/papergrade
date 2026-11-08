import { useCallback, useEffect, useRef, useState } from "react";
import { api, errorText } from "@/api";
import { OMRWorkerClient } from "@/scan/omrClient";
import { NumberTracker } from "@/scan/tracking";
import type { OMRPreview } from "@/scan/omr";
import { terminal } from "@/lib";
import type { CaptureResult, Exam, Manifest } from "@/types";
import { analyzeFrame, StabilityGate } from "@/scan/quality";
import { deleteJob, loadJobs, saveJob, type ScanJob } from "@/scan/store";
const MAX_QUEUE = 3;
export function useScanController(exam: Exam) {
  const video = useRef<HTMLVideoElement>(null),
    stream = useRef<MediaStream | null>(null),
    gate = useRef(new StabilityGate()),
    jobsRef = useRef<ScanJob[]>([]),
    busy = useRef(false),
    mounted = useRef(true),
    audio = useRef<AudioContext | null>(null),
    saving = useRef(false),
    runningRef = useRef(false),
    ready = useRef(false),
    soundRef = useRef(true),
    pauseRef = useRef(false),
    enqueueBusy = useRef(false),
    omr = useRef<OMRWorkerClient | null>(null),
    numberTracker = useRef(new NumberTracker());
  const [jobs, setJobs] = useState<ScanJob[]>([]),
    [running, setRunning] = useState(false),
    [hint, setHint] = useState(
      "启动一次相机，稳定后自动选帧；更换个人卡后继续",
    ),
    [error, setError] = useState(""),
    [sound, setSound] = useState(true),
    [active, setActive] = useState(""),
    [paused, setPaused] = useState(false),
    [manifest, setManifest] = useState<Manifest | null>(null),
    [starting, setStarting] = useState(false),
    [preview, setPreview] = useState<OMRPreview | null>(null);
  const sync = useCallback((next: ScanJob[]) => {
    jobsRef.current = next;
    if (mounted.current) setJobs([...next]);
  }, []);
  const stop = useCallback(() => {
    stream.current?.getTracks().forEach((t) => t.stop());
    stream.current = null;
    runningRef.current = false;
    setRunning(false);
  }, []);
  useEffect(() => {
    mounted.current = true;
    ready.current = false;
    loadJobs(exam.id)
      .then((items) => {
        if (!mounted.current) return;
        sync(items);
        gate.current = new StabilityGate(
          items.flatMap((j) =>
            !j.allowRescan && (j.trackingKey || j.token)
              ? [j.trackingKey || j.token!]
              : [],
          ),
        );
        ready.current = true;
      })
      .catch((e) => setError("浏览器持久队列不可用：" + errorText(e)));
    api<Manifest>("/exams/" + exam.id + "/template")
      .then(setManifest)
      .catch((e) => setError(errorText(e)));
    const hidden = () => {
      if (document.hidden) {
        stop();
        setHint("切到后台已释放相机；回到前台请点击继续。已保存任务可恢复。");
      }
    };
    document.addEventListener("visibilitychange", hidden);
    return () => {
      mounted.current = false;
      document.removeEventListener("visibilitychange", hidden);
      stream.current?.getTracks().forEach((t) => t.stop());
      runningRef.current = false;
      omr.current?.close();
      omr.current = null;
    };
  }, [exam.id, stop, sync]);
  const update = useCallback(
    async (job: ScanJob) => {
      await saveJob(job);
      sync(jobsRef.current.map((j) => (j.id === job.id ? job : j)));
    },
    [sync],
  );
  const chime = useCallback(() => {
    if (!soundRef.current || !audio.current) return;
    const context = audio.current;
    void context.resume();
    const oscillator = context.createOscillator(),
      gain = context.createGain();
    oscillator.connect(gain);
    gain.connect(context.destination);
    oscillator.frequency.setValueAtTime(880, context.currentTime);
    oscillator.frequency.setValueAtTime(1174, context.currentTime + 0.1);
    gain.gain.setValueAtTime(0.08, context.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.001, context.currentTime + 0.26);
    oscillator.start();
    oscillator.stop(context.currentTime + 0.27);
  }, []);
  useEffect(() => {
    if (paused) return;
    let cancelled = false;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    const tick = async () => {
      if (cancelled || busy.current || !navigator.onLine || document.hidden) {
        if (!cancelled) timer = setTimeout(tick, 1200);
        return;
      }
      const job = jobsRef.current.find(
        (j) => !j.error && (!j.result || !terminal(j.result.status)),
      );
      if (!job) {
        timer = setTimeout(tick, 900);
        return;
      }
      busy.current = true;
      setActive(job.id);
      try {
        let result: CaptureResult;
        if (job.captureID) {
          result = await api<CaptureResult>(
            "/captures/" + encodeURIComponent(job.captureID),
            { signal: controller.signal },
          );
        } else {
          const form = new FormData();
          form.set("image", job.blob, "capture.jpg");
          form.set("client_capture_id", job.id);
          if (job.token) form.set("candidate_sheet_token", job.token);
          result = await api<CaptureResult>("/exams/" + exam.id + "/captures", {
            method: "POST",
            headers: { "Idempotency-Key": job.id },
            body: form,
            signal: controller.signal,
          });
        }
        const previous = job.result?.status;
        const next = {
          ...job,
          blob: terminal(result.status) ? new Blob() : job.blob,
          captureID: result.capture_id,
          result,
          error: undefined,
        };
        await update(next);
        if (
          !cancelled &&
          result.status === "committed" &&
          previous !== "committed" &&
          !result.deduplicated
        )
          chime();
      } catch (e) {
        if (!cancelled)
          try {
            await update({ ...job, error: errorText(e) });
          } catch (storageError) {
            setError("无法保存任务状态：" + errorText(storageError));
          }
      } finally {
        busy.current = false;
        if (!cancelled) {
          setActive("");
          timer = setTimeout(tick, 1000);
        }
      }
    };
    void tick();
    return () => {
      cancelled = true;
      controller.abort();
      if (timer) clearTimeout(timer);
    };
  }, [exam.id, paused, update, chime]);
  const enqueue = useCallback(
    async (
      blob: Blob,
      token?: string,
      trackingKey?: string,
      localPreview?: OMRPreview,
    ) => {
      if (enqueueBusy.current) throw new Error("正在保存上一帧，请稍后再试");
      enqueueBusy.current = true;
      try {
        if (!ready.current) throw new Error("持久队列尚未就绪");
        if (blob.size > 4 * 1024 * 1024)
          throw new Error("图片超过 4MB，请压缩后上传");
        if (
          jobsRef.current.filter((j) => !j.result || !terminal(j.result.status))
            .length >= MAX_QUEUE
        )
          throw new Error("队列已满，请先处理已有任务");
        const job: ScanJob = {
          id: crypto.randomUUID(),
          examID: exam.id,
          token,
          trackingKey,
          preview: localPreview,
          blob,
          createdAt: Date.now(),
        };
        await saveJob(job);
        if (trackingKey || token) gate.current.reserve(trackingKey || token!);
        sync([...jobsRef.current, job]);
      } finally {
        enqueueBusy.current = false;
      }
    },
    [exam.id, sync],
  );
  const start = async () => {
    if (starting) return;
    setStarting(true);
    setError("");
    try {
      if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia)
        throw new Error(
          "相机需要 HTTPS 或本机 localhost。远程手机请部署 HTTPS，或使用图片上传。",
        );
      if (!ready.current)
        throw new Error("本机持久队列不可用，不能安全启动扫描");
      if (!manifest) throw new Error("模板尚未加载，暂不能启动");
      try {
        audio.current ??= new AudioContext();
        await audio.current.resume();
      } catch {
        setHint("浏览器未启用声音；仍以正式入库绿灯为准");
      }
      const media = await navigator.mediaDevices.getUserMedia({
        video: {
          facingMode: { ideal: "environment" },
          width: { ideal: 1920 },
          height: { ideal: 1440 },
        },
        audio: false,
      });
      if (!mounted.current) {
        media.getTracks().forEach((t) => t.stop());
        return;
      }
      stream.current = media;
      video.current!.srcObject = media;
      await video.current!.play();
      runningRef.current = true;
      setRunning(true);
    } catch (e) {
      stop();
      setError(errorText(e));
    } finally {
      setStarting(false);
    }
  };
  useEffect(() => {
    if (!running) return;
    const canvas = document.createElement("canvas");
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    const inspect = async () => {
      try {
        const v = video.current;
        if (!v || !v.videoWidth || !runningRef.current) return;
        if (saving.current || pauseRef.current) return;
        const pending = jobsRef.current.filter(
          (j) => !j.result || !terminal(j.result.status),
        ).length;
        if (pending >= MAX_QUEUE) {
          setHint("待处理队列已满，自动选帧暂停");
          return;
        }
        canvas.width = 480;
        canvas.height = Math.round((480 * v.videoHeight) / v.videoWidth);
        const ctx = canvas.getContext("2d", { willReadFrequently: true })!;
        ctx.drawImage(v, 0, 0, canvas.width, canvas.height);
        const v2 =
          manifest?.version === "a5-v2" || manifest?.version === "a5-v3";
        const frame = analyzeFrame(
          ctx.getImageData(0, 0, canvas.width, canvas.height),
          !v2,
        );
        const trackingKey = v2 ? "v2-framing-only" : frame.token;
        const gatedFrame = { ...frame, trackingKey };
        numberTracker.current.observe(frame.ready ? frame.corners : undefined);
        const accept =
          gate.current.inspect(gatedFrame) &&
          (!v2 || numberTracker.current.canInspect());
        setHint(
          (gatedFrame.trackingKey || gatedFrame.token) &&
            gate.current.has(gatedFrame.trackingKey || gatedFrame.token!)
            ? "当前卡已选帧，请换下一张个人卡（不会重复计数）"
            : frame.message,
        );
        if (accept && (gatedFrame.trackingKey || gatedFrame.token)) {
          saving.current = true;
          try {
            const output = document.createElement("canvas");
            const scale = Math.min(
              1,
              1800 / Math.max(v.videoWidth, v.videoHeight),
            );
            output.width = Math.round(v.videoWidth * scale);
            output.height = Math.round(v.videoHeight * scale);
            output
              .getContext("2d")!
              .drawImage(v, 0, 0, output.width, output.height);
            let localPreview: OMRPreview | undefined;
            if (v2 && manifest) {
              omr.current ??= new OMRWorkerClient();
              try {
                localPreview = await omr.current.recognize(
                  output
                    .getContext("2d")!
                    .getImageData(0, 0, output.width, output.height),
                  manifest,
                );
                setPreview(localPreview);
              } catch (e) {
                setHint(errorText(e));
                return;
              }
              if (localPreview.identity_state !== "recognized") {
                setHint("学号未确定，请修正填涂或用图片上传进入人工复核");
                return;
              }
              const key = "number:" + localPreview.student_number;
              if (gate.current.has(key)) {
                setHint(
                  "学号 " +
                    localPreview.student_number +
                    " 已选帧，请移走并换下一张不同学号卡",
                );
                return;
              }
            }
            const blob = await new Promise<Blob>((resolve, reject) =>
              output.toBlob(
                (b) => (b ? resolve(b) : reject(new Error("JPEG 编码失败"))),
                "image/jpeg",
                0.86,
              ),
            );
            if (!cancelled) {
              await enqueue(
                blob,
                v2 ? undefined : frame.token,
                v2 ? "number:" + localPreview!.student_number : frame.token,
                localPreview,
              );
              if (v2 && localPreview)
                numberTracker.current.captured(localPreview.corners);
              setHint("已选取清晰帧，正在上传；请移走并换下一张学号不同的卡");
            }
          } finally {
            saving.current = false;
          }
        }
      } catch (e) {
        if (!cancelled) setError(errorText(e));
      } finally {
        if (!cancelled) timer = setTimeout(inspect, 650);
      }
    };
    void inspect();
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [running, enqueue, manifest]);
  const retry = async (job: ScanJob) => {
    try {
      await update({ ...job, error: undefined });
    } catch (e) {
      setError(errorText(e));
    }
  };
  const allowRescan = async (job: ScanJob) => {
    try {
      await update({ ...job, allowRescan: true });
      if (job.trackingKey || job.token)
        gate.current.release(job.trackingKey || job.token!);
      numberTracker.current.rearm();
      setHint("已允许此识别失败卡重新选帧，请保持清晰稳定");
    } catch (e) {
      setError(errorText(e));
    }
  };
  const discard = async (job: ScanJob) => {
    if (job.captureID) {
      setError("服务器已受理任务，不能删除来规避幂等。请等待结果或重新查询。");
      return;
    }
    if (
      !confirm(
        "移除此本地任务？尚未确认服务器是否已接收；重新拍摄可能返回重复或冲突。",
      )
    )
      return;
    try {
      await deleteJob(job.id);
      if (job.trackingKey || job.token)
        gate.current.release(job.trackingKey || job.token!);
      sync(jobsRef.current.filter((j) => j.id !== job.id));
    } catch (e) {
      setError(errorText(e));
    }
  };
  const committed = jobs.filter(
    (j) => j.result?.status === "committed" && !j.result.deduplicated,
  ).length;

  return {
    video,
    preview,
    manifest,
    async inspectUpload(blob: Blob) {
      if (manifest?.version !== "a5-v2" && manifest?.version !== "a5-v3")
        return undefined;
      const bitmap = await createImageBitmap(blob);
      const canvas = document.createElement("canvas");
      canvas.width = bitmap.width;
      canvas.height = bitmap.height;
      canvas.getContext("2d")!.drawImage(bitmap, 0, 0);
      bitmap.close();
      omr.current ??= new OMRWorkerClient();
      const result = await omr.current.recognize(
        canvas
          .getContext("2d")!
          .getImageData(0, 0, canvas.width, canvas.height),
        manifest,
      );
      setPreview(result);
      return result;
    },
    jobs,
    running,
    hint,
    error,
    setError,
    sound,
    setSound,
    soundRef,
    active,
    paused,
    setPaused,
    pauseRef,
    starting,
    start,
    stop,
    enqueue,
    retry,
    discard,
    allowRescan,
    committed,
  };
}
