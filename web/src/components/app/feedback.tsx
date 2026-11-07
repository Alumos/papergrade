import type { ReactNode } from "react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { statusLabel } from "@/lib";
export function Feedback({
  children,
  error = false,
}: {
  children: ReactNode;
  error?: boolean;
}) {
  return (
    <Alert variant={error ? "destructive" : "default"}>
      <AlertTitle>{error ? "操作未完成" : "使用提示"}</AlertTitle>
      <AlertDescription>{children}</AlertDescription>
    </Alert>
  );
}
export function Loading() {
  return (
    <div className="grid gap-4" aria-label="正在加载">
      <Skeleton className="h-10 w-1/3" />
      <Skeleton className="h-48 w-full" />
    </div>
  );
}
export function Empty({ children }: { children: ReactNode }) {
  return (
    <Card>
      <CardContent className="py-12 text-center text-sm text-muted-foreground">
        {children}
      </CardContent>
    </Card>
  );
}
export function Status({ value }: { value: string }) {
  return (
    <Badge
      variant={
        ["failed", "conflict"].includes(value) ? "destructive" : "secondary"
      }
      className={value === "committed" ? "bg-green-100 text-green-800" : ""}
    >
      {statusLabel[value] ?? value}
    </Badge>
  );
}
export function PageTitle({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        <p className="text-sm text-muted-foreground">{description}</p>
      </div>
      {action}
    </div>
  );
}
