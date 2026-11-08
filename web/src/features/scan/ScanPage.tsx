import { useRef } from "react";
import {
  Camera,
  Pause,
  Play,
  Upload,
  Volume2,
  VolumeX,
  RefreshCw,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Feedback, Status } from "@/components/app/feedback";
import { errorText } from "@/api";
import { score, terminal } from "@/lib";
import type { Exam } from "@/types";
import { useScanController } from "./useScanController";
export default function ScanPage({ exam }: { exam: Exam }) {
  const scan = useScanController(exam);
  const input = useRef<HTMLInputElement>(null);
  const upload = async (file: File) => {
    try {
      if (file.size > 20 * 1024 * 1024) throw new Error("原始图片不能超过20MB");
      const bitmap = await createImageBitmap(file),
        canvas = document.createElement("canvas"),
        scale = Math.min(1, 1800 / Math.max(bitmap.width, bitmap.height));
      canvas.width = Math.round(bitmap.width * scale);
      canvas.height = Math.round(bitmap.height * scale);
      canvas
        .getContext("2d")!
        .drawImage(bitmap, 0, 0, canvas.width, canvas.height);
      bitmap.close();
      const blob = await new Promise<Blob>((resolve, reject) =>
        canvas.toBlob(
          (b) => (b ? resolve(b) : reject(new Error("图片编码失败"))),
          "image/jpeg",
          0.88,
        ),
      );
      let preview;
      try {
        preview = await scan.inspectUpload(blob);
      } catch (e) {
        scan.setError(
          "本机预览未通过：" + errorText(e) + "。仍可交服务端独立复核。",
        );
      }
      await scan.enqueue(
        blob,
        undefined,
        preview?.identity_state === "recognized"
          ? "number:" + preview.student_number
          : undefined,
        preview,
      );
    } catch (e) {
      scan.setError(errorText(e));
    }
  };
  return (
    <div className="space-y-5">
      {scan.error && <Feedback error>{scan.error}</Feedback>}
      {scan.preview && (
        <Feedback>
          本机WebWorker预览（未入库）：学号{" "}
          {scan.preview.student_number || "未确定"} · 身份{" "}
          {scan.preview.identity_state} ·{" "}
          {scan.preview.answers
            .map(
              (a) =>
                (scan.manifest?.questions?.find((q) => q.id === a.question_id)
                  ?.number ?? a.question_id) +
                ":" +
                (a.selected.join("/") || a.state),
            )
            .join("；")}
          。仅服务端committed才成功。
        </Feedback>
      )}
      <div className="grid gap-5 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>连续扫描</CardTitle>
              <Badge variant="outline">本机确认 {scan.committed} 份</Badge>
            </div>
            <CardDescription>
              稳定后自动选帧；新卡使用学号填涂+画面位移换卡，不使用全班相同二维码做身份锁。
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="relative aspect-[3/4] max-h-[560px] overflow-hidden rounded-lg bg-neutral-950">
              <video
                ref={scan.video}
                playsInline
                muted
                className="absolute inset-0 h-full w-full object-contain"
                aria-label="相机取景"
              />
              {!scan.running && (
                <div className="absolute inset-0 grid place-content-center gap-3 text-center text-neutral-400">
                  <Camera className="mx-auto size-9" />
                  <p className="text-sm">将完整答题卡放入镜头</p>
                </div>
              )}
              <div className="pointer-events-none absolute inset-[12%] rounded-lg border border-dashed border-white/40" />
              <Badge className="absolute left-3 top-3" variant="secondary">
                {scan.running ? "相机已启动" : "相机已暂停"}
              </Badge>
            </div>
            <p className="min-h-10 text-sm" role="status">
              {scan.hint}
            </p>
            <div className="flex flex-wrap gap-2">
              {scan.running ? (
                <Button
                  className="min-h-11"
                  variant="outline"
                  onClick={scan.stop}
                >
                  <Pause />
                  暂停相机
                </Button>
              ) : (
                <Button
                  className="min-h-11"
                  disabled={scan.starting}
                  onClick={() => void scan.start()}
                >
                  <Camera />
                  {scan.starting ? "启动中" : "启动 / 继续"}
                </Button>
              )}
              <Button
                className="min-h-11"
                variant="outline"
                onClick={() => input.current?.click()}
              >
                <Upload />
                上传图片
              </Button>
              <input
                ref={input}
                type="file"
                accept="image/jpeg,image/png"
                className="hidden"
                aria-label="上传答题卡图片"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  e.target.value = "";
                  if (file) void upload(file);
                }}
              />
              <Button
                className="min-h-11"
                variant="ghost"
                size="icon"
                aria-label={scan.sound ? "关闭提示音" : "开启提示音"}
                onClick={() => {
                  scan.soundRef.current = !scan.sound;
                  scan.setSound(!scan.sound);
                }}
              >
                {scan.sound ? <Volume2 /> : <VolumeX />}
              </Button>
            </div>
            <p className="text-xs leading-6 text-muted-foreground">
              小画布每650ms检测质量、二维码与稳定性，不定时盲拍。帧不存 React
              state；压缩 JPEG 后由服务端 / 云端独立验证。
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <div className="flex justify-between gap-2 items-center">
              <CardTitle>处理队列</CardTitle>
              <Button
                className="min-h-11"
                variant="outline"
                onClick={() => {
                  scan.pauseRef.current = !scan.paused;
                  scan.setPaused(!scan.paused);
                }}
              >
                {scan.paused ? <Play /> : <Pause />}
                {scan.paused ? "恢复队列" : "暂停队列"}
              </Button>
            </div>
            <CardDescription>
              最多3项待处理。已发送的请求不会因暂停而撤销。
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {scan.paused && (
              <Feedback>已暂停上传与自动选帧，恢复后按原幂等键继续。</Feedback>
            )}
            <div className="text-xs text-muted-foreground">
              待处理{" "}
              {
                scan.jobs.filter((j) => !j.result || !terminal(j.result.status))
                  .length
              }{" "}
              / 3
            </div>
            {!scan.jobs.length && (
              <p className="py-12 text-center text-sm text-muted-foreground">
                尚无扫描任务，拍摄后先保存到本机。
              </p>
            )}
            <div className="max-h-[650px] space-y-4 overflow-y-auto">
              {[...scan.jobs].reverse().map((job, i) => (
                <div key={job.id} className="space-y-3">
                  <Separator />
                  <div className="flex justify-between gap-3">
                    <div>
                      <p className="font-medium text-sm">
                        {job.result?.student_display ||
                          "答题卡 " + (scan.jobs.length - i)}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {new Date(job.createdAt).toLocaleTimeString("zh-CN")} ·{" "}
                        {job.id.slice(0, 8)}
                      </p>
                    </div>
                    <Status
                      value={
                        job.error
                          ? "retry"
                          : (job.result?.status ??
                            (scan.active === job.id ? "uploading" : "local"))
                      }
                    />
                  </div>
                  {(job.result?.student_number ||
                    job.result?.identity_state) && (
                    <p className="text-sm text-muted-foreground">
                      服务端学号 {job.result.student_number || "未绑定"} · 身份{" "}
                      {job.result.identity_state}
                    </p>
                  )}
                  {job.result?.status === "committed" && (
                    <p className="text-sm font-medium text-green-700">
                      正式入库 · {score(job.result.score_units)} /{" "}
                      {score(job.result.max_score_units)} 分
                    </p>
                  )}
                  {(job.error || job.result?.message) && (
                    <p className="text-sm text-muted-foreground">
                      {job.error || job.result?.message}
                    </p>
                  )}
                  {job.error && (
                    <div className="flex gap-2">
                      <Button
                        className="min-h-11"
                        variant="outline"
                        onClick={() => void scan.retry(job)}
                      >
                        <RefreshCw />
                        原请求重试
                      </Button>
                      {!job.captureID && (
                        <Button
                          className="min-h-11"
                          variant="ghost"
                          onClick={() => void scan.discard(job)}
                        >
                          移除本地任务
                        </Button>
                      )}
                    </div>
                  )}
                  {job.result?.status === "failed" &&
                    job.token &&
                    !job.allowRescan && (
                      <Button
                        className="min-h-11"
                        variant="outline"
                        onClick={() => void scan.allowRescan(job)}
                      >
                        允许此卡重新选帧
                      </Button>
                    )}
                </div>
              ))}
            </div>
            <Separator />
            <p className="text-xs leading-6 text-muted-foreground">
              仅 committed 绿灯，新入库才发声；duplicate
              不计数，待复核不算成功。刷新恢复 IndexedDB
              队列，后台释放相机，前台点击继续。
            </p>
          </CardContent>
        </Card>
      </div>
      <Feedback>
        固定A4双联卡试验版，沿中线裁切后只拍一张半A4卡。v2默认服务端学号OMR，不依赖云端Card网关；旧卡仍按旧能力。真实手机2B、反光折痕和复杂透视尚需实测，相机须HTTPS或本机localhost。
      </Feedback>
    </div>
  );
}
