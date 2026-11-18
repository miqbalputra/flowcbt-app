// Klien API ujian (backend S9). Semuanya lewat lib/api.
import { api, ApiError } from "../../lib/api";

export { ApiError };

/** Pesan error siap-tampil. */
export const errorOf = (e: unknown): string =>
  e instanceof ApiError ? e.message : "Terjadi kesalahan. Coba lagi.";

export interface Exam {
  id: string;
  question_set_id: string;
  set_version_id?: string;
  name: string;
  status: "draft" | "published" | "paused" | "closed" | "cancelled";
  open_at?: string;   // ISO dengan offset (WIB default di FE)
  close_at?: string;
  duration_minutes?: number;
  close_policy: "hard" | "grace";
  login_mode: "account" | "guest" | "both";
  max_attempts?: number;
  score_policy: "highest" | "last" | "avg";
  passing_score?: number;
  show_score_immediately: boolean;
  show_review: boolean;
  show_explanations: boolean;
  shuffle_questions: boolean;
  shuffle_options: boolean;
  free_navigation: boolean;
  require_all_answered: boolean;
  created_at: string;
  updated_at: string;
}

export type ExamDraft = {
  question_set_id: string;
  name: string;
  open_at: string;
  close_at: string;
  duration_minutes: number;
  close_policy: Exam["close_policy"];
  login_mode: Exam["login_mode"];
  max_attempts: number | null;
  score_policy: Exam["score_policy"];
  passing_score: number | null;
  show_score_immediately?: boolean;
  show_review?: boolean;
  show_explanations?: boolean;
  shuffle_questions?: boolean;
  shuffle_options?: boolean;
  free_navigation?: boolean;
  require_all_answered?: boolean;
};

export interface Room {
  id: string;
  exam_id: string;
  name: string;
}

export interface Token {
  token: string;
  exam_id: string;
  room_id?: string | null;
  is_active: boolean;
}

export interface Participant {
  student_id: string;
  student_code: string;
  name: string;
  class_name?: string | null;
  source: "assignment" | "manual" | "guest";
  extra_time_seconds: number;
  attempt_status?: string;
  attempts: number;
  violations: number;
}

export type Assignment = {
  target_type: "class" | "student";
  class_id?: string;
  student_id?: string;
};

export const listExams = (q = "") =>
  api<Exam[]>(`/exams${q ? `?q=${encodeURIComponent(q)}` : ""}`);

export const getExam = (id: string) => api<Exam>(`/exams/${id}`);

export const createExam = (body: ExamDraft) =>
  api<Exam>("/exams", { method: "POST", body });

export const updateExam = (id: string, body: Partial<ExamDraft>) =>
  api(`/exams/${id}`, { method: "PUT", body });

export const deleteExam = (id: string) =>
  api(`/exams/${id}`, { method: "DELETE" });

export const listRooms = (examId: string) => api<Room[]>(`/exams/${examId}/rooms`);

export const addRoom = (examId: string, name: string) =>
  api<Room>(`/exams/${examId}/rooms`, { method: "POST", body: { name } });

export const listTokens = (examId: string) => api<Token[]>(`/exams/${examId}/tokens`);

export const issueToken = (examId: string, roomId?: string) =>
  api<Token>(`/exams/${examId}/tokens`, { method: "POST", body: { room_id: roomId ?? "" } });

export const revokeToken = (token: string) =>
  api(`/exam-tokens/${encodeURIComponent(token)}`, { method: "DELETE" });

export const setAssignments = (
  examId: string,
  assignments: { target_type: "class" | "student"; class_id?: string; student_id?: string }[],
) => api(`/exams/${examId}/assignments`, { method: "PUT", body: { assignments } });

export const listParticipants = (examId: string) =>
  api<Participant[]>(`/exams/${examId}/participants`);

// ---- Siklus status & aksi pengawas (S10, FR-5.7) ----

export const publishExam = (id: string) =>
  api<Exam>(`/exams/${id}/publish`, { method: "POST" });

export const setExamStatus = (id: string, action: "pause" | "resume" | "close" | "cancel") =>
  api<Exam>(`/exams/${id}/status`, { method: "POST", body: { action } });

export const listAssignments = (examId: string) =>
  api<Assignment[]>(`/exams/${examId}/assignments`);

export const regenerateTokens = (examId: string, roomId?: string) =>
  api<Token>(`/exams/${examId}/tokens/regenerate`, { method: "POST", body: { room_id: roomId ?? "" } });

export const addTime = (examId: string, studentId: string, seconds: number) =>
  api(`/exams/${examId}/participants/${studentId}/add-time`, { method: "POST", body: { seconds } });

export const resetAttempts = (examId: string, studentId: string) =>
  api(`/exams/${examId}/participants/${studentId}/reset-attempts`, { method: "POST" });

export const forceSubmit = (examId: string, studentId: string) =>
  api(`/exams/${examId}/participants/${studentId}/force-submit`, { method: "POST" });

export const kickSession = (examId: string, studentId: string) =>
  api(`/exams/${examId}/participants/${studentId}/kick`, { method: "POST" });

/** datetime-local ("2026-01-10T07:00") → RFC3339 WIB default (PRD FR-5.1). */
export const toWIB = (dtLocal: string): string => {
  if (!dtLocal) return "";
  const [date, time] = dtLocal.split("T");
  if (!time) return "";
  const [h, m] = time.split(":");
  return `${date}T${h}:${m}:00+07:00`;
};
