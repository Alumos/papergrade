export function parseRoster(text: string): {
    number: string;
    name: string;
}[] {
    const rows: string[][] = [];
    let row: string[] = [], cell = '', quoted = false;
    for (let i = 0; i < text.length; i++) {
        const c = text[i];
        if (c === '"') {
            if (quoted && text[i + 1] === '"') {
                cell += '"';
                i++;
            }
            else if (!quoted && cell.trim() !== '')
                throw new Error('CSV 引号格式错误');
            else
                quoted = !quoted;
        }
        else if (!quoted && (c === ',' || c === '\t')) {
            row.push(cell.trim());
            cell = '';
        }
        else if (!quoted && (c === '\n' || c === '\r')) {
            if (c === '\r' && text[i + 1] === '\n')
                i++;
            row.push(cell.trim());
            if (row.some(Boolean))
                rows.push(row);
            row = [];
            cell = '';
        }
        else
            cell += c;
    }
    if (quoted)
        throw new Error('CSV 引号未闭合');
    row.push(cell.trim());
    if (row.some(Boolean))
        rows.push(row);
    if (rows.length && ['学号', 'number', 'student_number'].includes(rows[0][0].replace(/^\uFEFF/, '').toLowerCase()))
        rows.shift();
    const seen = new Set<string>();
    return rows.map((r, i) => { r[0] = r[0]?.replace(/^\uFEFF/, ''); if (r.length !== 2 || !r[0] || !r[1])
        throw new Error('第 ' + (i + 1) + ' 行应为学号,姓名'); if (seen.has(r[0]))
        throw new Error('学号重复：' + r[0]); seen.add(r[0]); return { number: r[0], name: r[1] }; });
}
export const score = (units: number | undefined) => units === undefined ? '—' : (units / 100).toLocaleString('zh-CN', { maximumFractionDigits: 2 });
export const terminal = (status: string) => ['committed', 'needs_review', 'duplicate', 'conflict', 'failed'].includes(status);
export const statusLabel: Record<string, string> = { queued: '等待识别', processing: '识别中', committed: '已正式入库', needs_review: '待教师复核', duplicate: '重复卡 · 未计数', conflict: '答卷冲突 · 未覆盖', failed: '识别失败', uploading: '上传中', local: '等待上传', retry: '连接中断', draft: '草稿', published: '已发布', pending: '等待处理', uploaded: '已上传', parsing: '解析中', parsed: '解析完成', completed: '处理完成' };
