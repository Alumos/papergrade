import type { CaptureResult } from '../types';
export type ScanJob = {
    id: string;
    examID: string;
    token?: string;
    blob: Blob;
    createdAt: number;
    captureID?: string;
    result?: CaptureResult;
    error?: string;
    allowRescan?: boolean;
};
function database(): Promise<IDBDatabase> { return new Promise((resolve, reject) => { const request = indexedDB.open('papergrade-scan', 2); request.onupgradeneeded = () => { const store = request.result.objectStoreNames.contains('jobs') ? request.transaction!.objectStore('jobs') : request.result.createObjectStore('jobs', { keyPath: 'id' }); if (!store.indexNames.contains('examID'))
    store.createIndex('examID', 'examID'); }; request.onsuccess = () => resolve(request.result); request.onerror = () => reject(request.error); }); }
export async function saveJob(job: ScanJob) { const db = await database(); try {
    await new Promise<void>((resolve, reject) => { const tx = db.transaction('jobs', 'readwrite'); tx.objectStore('jobs').put(job); tx.oncomplete = () => resolve(); tx.onerror = () => reject(tx.error); tx.onabort = () => reject(tx.error); });
}
finally {
    db.close();
} }
export async function loadJobs(examID: string): Promise<ScanJob[]> { const db = await database(); try {
    return await new Promise((resolve, reject) => { const request = db.transaction('jobs').objectStore('jobs').index('examID').getAll(examID); request.onsuccess = () => resolve((request.result as ScanJob[]).filter(j => j.examID === examID).sort((a, b) => a.createdAt - b.createdAt)); request.onerror = () => reject(request.error); });
}
finally {
    db.close();
} }
export async function deleteJob(id: string) { const db = await database(); try {
    await new Promise<void>((resolve, reject) => { const tx = db.transaction('jobs', 'readwrite'); tx.objectStore('jobs').delete(id); tx.oncomplete = () => resolve(); tx.onerror = () => reject(tx.error); });
}
finally {
    db.close();
} }
