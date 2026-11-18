// Klien API bank soal (backend S6+S8). Semua lewat lib/api.
import { api, apiBlob, ApiError, downloadBlob } from "../../lib/api";

export { ApiError, downloadBlob };

/** Pesan error siap-tampil untuk throw unknown. */
export const errorOf = (e: unknown): string =>
  e instanceof ApiError ? e.message : "Terjadi kesalahan. Coba lagi.";

export interface Option {
  id: string;
  html: string;
  correct: boolean;
}

export interface Question {
  id: string;
  order_no: number;
  type: "mcq" | "mcq_complex" | "matching" | "short_answer" | "essay";
  stimulus_id?: string | null;
  body: string; // rich text tersanitasi
  options?: Option[];
  answer_key?: unknown; // TIDAK dikirim ke klien siswa; hanya untuk editor guru
  rubric?: unknown;
  points: number;
  scoring_mode: "all_or_nothing" | "partial";
  partial_wrong_penalty: boolean;
  shuffle_options: boolean;
  explanation_html?: string;
  media_ids?: string[];
}

export interface Stimulus {
  id: string;
  order_no: number;
  title?: string;
  content_html: string;
  media_ids?: string[];
}

export interface QuestionSet {
  id: string;
  subject_name: string;
  title: string;
  description?: string;
  exam_code: string;
  status: "draft" | "published" | "archived";
  level_tags: string[];
  shared_with_tenant: boolean;
  question_count?: number; // backend kirim versi, bukan di row paket
  max_points?: number;
  created_at: string;
  updated_at: string;
  stimuli?: Stimulus[];
}

export const listSets = (q = "") =>
  api<QuestionSet[]>(`/question-sets${q ? `?q=${encodeURIComponent(q)}` : ""}`);

export const getSet = (id: string) =>
  api<QuestionSet>(`/question-sets/${id}`);

export const createSet = (body: Pick<QuestionSet, "subject_name" | "title" | "description"> & { exam_code?: string }) =>
  api<{ id: string }>("/question-sets", { method: "POST", body });

export const getSetQuestions = (id: string) =>
  api<Question[]>(`/question-sets/${id}/questions`);

export const createQuestion = (setId: string, q: Partial<Question>) =>
  api<{ id: string }>(`/question-sets/${setId}/questions`, { method: "POST", body: q });

export const updateQuestion = (qid: string, q: Partial<Question>) =>
  api(`/questions/${qid}`, { method: "PUT", body: q });

export const deleteQuestion = (qid: string) =>
  api(`/questions/${qid}`, { method: "DELETE" });

export const reorderQuestions = (setId: string, order: string[]) =>
  api("/questions/reorder", { method: "POST", body: { set_id: setId, order } });

export const publishSet = (id: string) =>
  api<QuestionSet>(`/question-sets/${id}/publish`, { method: "POST" });

export const unpublishSet = (id: string) =>
  api<{ message: string }>(`/question-sets/${id}/unpublish`, { method: "POST" });

export const duplicateSet = (id: string) =>
  api<{ id: string }>(`/question-sets/${id}/duplicate`, { method: "POST" });

export const archiveSet = (id: string) =>
  api(`/question-sets/${id}`, { method: "DELETE" });

// Media: gambar & audio stimulus.
export interface MediaAsset {
  id: string;
  kind: "image" | "audio";
  url: string;
  mime: string;
  size_bytes: number;
}

export const uploadMedia = async (file: File, kind: "image" | "audio" = "image") => {
  const fd = new FormData();
  fd.append("file", file);
  fd.append("kind", kind);
  return api<MediaAsset>("/media", { method: "POST", body: fd });
};

// ---- import/export template soal (S8) ----

export interface QuestionImportError {
  row: number;
  reason: string;
}

export interface QuestionImportPreview {
  total: number;
  valid: Question[];
  errors: QuestionImportError[];
}

export const downloadQuestionTemplate = () =>
  apiBlob("/question-sets/import/template").then((b) => downloadBlob(b, "template-soal-flowcbt.xlsx"));

export const previewQuestionImport = (setId: string, file: File): Promise<QuestionImportPreview> => {
  const fd = new FormData();
  fd.append("file", file);
  return api<QuestionImportPreview>(`/question-sets/${setId}/import/preview`, { method: "POST", body: fd });
};

export const commitQuestionImport = (setId: string, file: File) => {
  const fd = new FormData();
  fd.append("file", file);
  return api<{ inserted: number; errors: QuestionImportError[] }>(
    `/question-sets/${setId}/import/commit`, { method: "POST", body: fd });
};

export const exportQuestions = (setId: string, format: "xlsx" | "csv" = "xlsx") =>
  apiBlob(`/question-sets/${setId}/export?format=${format}`).then((b) =>
    downloadBlob(b, `soal-export.${format}`));
