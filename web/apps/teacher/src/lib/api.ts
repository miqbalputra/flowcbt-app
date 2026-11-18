// Klien API FlowCBT (PRD 9): base /api/v1, Bearer JWT, refresh otomatis sekali
// pada 401. Semua akses jaringan aplikasi guru lewat modul ini.

const BASE = "/api/v1";
const ACCESS_KEY = "flowcbt:access";
const REFRESH_KEY = "flowcbt:refresh";

export function getAccessToken(): string | null {
  return sessionStorage.getItem(ACCESS_KEY);
}

export function setTokens(access: string, refresh: string) {
  sessionStorage.setItem(ACCESS_KEY, access);
  // Catatan dev: sebelum rilis publik, refresh token dipindah ke cookie httpOnly
  // di sisi server (FR-1.6); localStorage dipakai agar alur dev sederhana.
  localStorage.setItem(REFRESH_KEY, refresh);
}

export function clearTokens() {
  sessionStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

export class ApiError extends Error {
  code: string;
  status: number;
  details?: unknown;
  constructor(status: number, code: string, message: string, details?: unknown) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  skipAuth?: boolean;
  retry?: boolean;
}

async function tryRefresh(): Promise<boolean> {
  const refresh = localStorage.getItem(REFRESH_KEY);
  if (!refresh) return false;
  const res = await fetch(`${BASE}/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refresh }),
  });
  if (!res.ok) {
    clearTokens();
    return false;
  }
  const data = (await res.json()) as { access_token: string; refresh_token: string };
  setTokens(data.access_token, data.refresh_token);
  return true;
}

export async function api<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const isForm = opts.body instanceof FormData;
  const headers: Record<string, string> = isForm
    ? {} // FormData: browser set Content-Type multipart + boundary sendiri
    : { "Content-Type": "application/json" };
  const token = getAccessToken();
  if (!opts.skipAuth && token) headers.Authorization = `Bearer ${token}`;

  const res = await fetch(`${BASE}${path}`, {
    method: opts.method ?? "GET",
    headers,
    body: opts.body === undefined ? undefined : isForm ? (opts.body as FormData) : JSON.stringify(opts.body),
  });

  // Access token kadaluarsa → refresh lalu ulang sekali.
  if (res.status === 401 && !opts.skipAuth && opts.retry !== false) {
    if (await tryRefresh()) {
      return api<T>(path, { ...opts, retry: false });
    }
  }

  if (!res.ok) {
    let code = "REQUEST_FAILED";
    let message = `Permintaan gagal (${res.status})`;
    let details: unknown;
    try {
      const err = (await res.json()) as { code?: string; message?: string; details?: unknown };
      code = err.code ?? code;
      message = err.message ?? message;
      details = err.details;
    } catch {
      // body non-JSON: pakai default di atas
    }
    throw new ApiError(res.status, code, message, details);
  }
  return (await res.json()) as T;
}

/** Unduh berkas (template/export) dengan auth; melempar ApiError bila gagal. */
export async function apiBlob(path: string): Promise<Blob> {
  const token = getAccessToken();
  const headers: Record<string, string> = token ? { Authorization: `Bearer ${token}` } : {};
  const res = await fetch(`${BASE}${path}`, { headers });
  if (!res.ok) {
    let message = `Permintaan gagal (${res.status})`;
    try {
      const err = (await res.json()) as { message?: string };
      message = err.message ?? message;
    } catch {
      // body bukan JSON: pakai default
    }
    throw new ApiError(res.status, "REQUEST_FAILED", message);
  }
  return res.blob();
}

/** Picu unduhan berkas blob di browser. */
export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
