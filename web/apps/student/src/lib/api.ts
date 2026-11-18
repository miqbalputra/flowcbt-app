// Klien API siswa (S11): base /api/v1, Bearer JWT, refresh otomatis sekali.
// Kunci storage berbeda dari aplikasi guru agar dua app bisa diuji di browser sama.

const BASE = "/api/v1";
const ACCESS_KEY = "flowcbt:student:access";
const REFRESH_KEY = "flowcbt:student:refresh";

export function getAccessToken(): string | null {
  return sessionStorage.getItem(ACCESS_KEY);
}

export function setTokens(access: string, refresh: string) {
  sessionStorage.setItem(ACCESS_KEY, access);
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
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const token = getAccessToken();
  if (!opts.skipAuth && token) headers.Authorization = `Bearer ${token}`;

  const res = await fetch(`${BASE}${path}`, {
    method: opts.method ?? "GET",
    headers,
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
  });

  if (res.status === 401 && !opts.skipAuth && opts.retry !== false) {
    if (await tryRefresh()) {
      return api<T>(path, { ...opts, retry: false });
    }
  }

  if (!res.ok) {
    let code = "REQUEST_FAILED";
    let message = `Permintaan gagal (${res.status})`;
    try {
      const err = (await res.json()) as { code?: string; message?: string };
      code = err.code ?? code;
      message = err.message ?? message;
    } catch {
      // body bukan JSON: pakai default
    }
    throw new ApiError(res.status, code, message);
  }
  return (await res.json()) as T;
}

export const errorOf = (e: unknown): string =>
  e instanceof ApiError ? e.message : "Terjadi kesalahan. Coba lagi.";

// ---- endpoint S11 ----

export interface StudentExam {
  id: string;
  name: string;
  status: "published" | "paused";
  open_at?: string | null;
  close_at?: string | null;
  duration_minutes?: number;
  login_mode: "account" | "guest" | "both";
  attempts_done: number;
  max_attempts?: number | null;
  extra_time_seconds: number;
  active_attempt_id?: string;
  show_score_immediately: boolean;
}

export interface ClientQuestion {
  id: string;
  order_no: number;
  type: "mcq" | "mcq_complex" | "matching" | "short_answer" | "essay";
  stimulus_id?: string | null;
  body: string;
  points: number;
  options?: { id: string; html: string }[];
  matching?: { left: { id: string; html: string }[]; right: { id: string; html: string }[] };
  shuffle_options: boolean;
  media_ids?: string[];
}

export interface StartResult {
  attempt_id: string;
  exam_id: string;
  exam_name: string;
  deadline_at: string;
  duration_seconds: number;
  server_time: string;
  show_score_immediately: boolean;
  shuffle_questions: boolean;
  free_navigation: boolean;
  require_all_answered: boolean;
  questions: ClientQuestion[];
}

export interface Tokens {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

export const studentLogin = (school_code: string, username: string, password: string) =>
  api<Tokens>("/auth/student-login", { method: "POST", body: { school_code, username, password }, skipAuth: true });

export const guestAuth = (token: string) =>
  api<Tokens & { exam_id: string }>("/exams/guest-auth", { method: "POST", body: { token }, skipAuth: true });

export const myExams = () => api<StudentExam[]>("/me/exams");

export const startExam = (examId: string, device_hash = "web") =>
  api<StartResult>(`/exams/${examId}/start`, { method: "POST", body: { device_hash } });
