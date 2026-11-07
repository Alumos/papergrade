export class ApiError extends Error {
    constructor(public status: number, public code: string, message: string) { super(message); }
}
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
    const controller = new AbortController();
    const cloudMutation = init.method === 'POST' && /\/(captures|parse)$/.test(path);
    const timer = setTimeout(() => controller.abort(), cloudMutation ? 115000 : 25000);
    const abort = () => controller.abort();
    if (init.signal?.aborted)
        controller.abort();
    init.signal?.addEventListener('abort', abort, { once: true });
    try {
        const headers = new Headers(init.headers);
        if (init.body && !(init.body instanceof FormData))
            headers.set('Content-Type', 'application/json');
        const response = await fetch('/api/v1' + path, { ...init, headers, credentials: 'same-origin', signal: controller.signal });
        if (!response.ok) {
            const body = await response.json().catch(() => ({}));
            if (response.status === 401 && !['/me', '/auth/login'].includes(path))
                window.dispatchEvent(new Event('papergrade:session-expired'));
            throw new ApiError(response.status, body.code ?? 'HTTP_ERROR', body.message ?? '请求失败，请稍后重试');
        }
        if (response.status === 204)
            return undefined as T;
        return await response.json() as T;
    }
    finally {
        clearTimeout(timer);
        init.signal?.removeEventListener('abort', abort);
    }
}
export const post = <T>(path: string, body: unknown) => api<T>(path, { method: 'POST', body: JSON.stringify(body) });
export function errorText(error: unknown) { return error instanceof Error ? error.message : '发生未知错误'; }
export async function printCards(examID: string, studentIDs: string[]) {
    const popup = window.open('about:blank', '_blank');
    if (!popup)
        throw new Error('请允许弹出窗口以打印答题卡');
    popup.document.title = '个人答题卡';
    popup.document.body.textContent = '正在加载答题卡…';
    try {
        const sources: string[] = [];
        // Fetch serially: printing a large class must not fan out work on a small API server.
        for (const id of studentIDs) {
            const r = await fetch('/api/v1/exams/' + encodeURIComponent(examID) + '/cards?student_id=' + encodeURIComponent(id), { credentials: 'same-origin' });
            if (!r.ok) throw new Error('答题卡加载失败 (' + r.status + ')');
            sources.push('data:image/svg+xml;charset=utf-8,' + encodeURIComponent(await r.text()));
        }
        popup.document.body.textContent = '';
        const style = popup.document.createElement('style');
        style.textContent = '@page{size:A4;margin:0}body{margin:0}img{display:block;width:210mm;height:297mm;break-after:page}';
        popup.document.head.append(style);
        await Promise.all(sources.map(src => new Promise<void>((resolve, reject) => { const img = popup.document.createElement('img'); img.onload = () => resolve(); img.onerror = () => reject(new Error('答题卡图像无法显示')); img.src = src; popup.document.body.append(img); })));
        popup.focus();
        popup.print();
    }
    catch (e) {
        popup.close();
        throw e;
    }
}
