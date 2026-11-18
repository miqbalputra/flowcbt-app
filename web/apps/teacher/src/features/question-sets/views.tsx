// Slot editor umum per tipe soal: komponen presentasi murni (tanpa efek samping),
// dipakai EditorPage untuk semua 5 tipe. HTML di sini sudah tersanitasi server.
import type { Option, Question } from "./api";

export function QuestionBodyView({ q }: { q: Question }) {
  return <div className="rich" dangerouslySetInnerHTML={{ __html: q.body }} />;
}

export function QuestionPointsView({ q }: { q: Question }) {
  return (
    <span className="points" title="poin">
      {q.points}
    </span>
  );
}

export function OptionsView({ q, shuffle }: { q: Question; shuffle?: boolean }) {
  const opts = q.options ?? [];
  const shown = shuffle
    ? [...opts].map((o, i) => ({ o, k: (i * 2654435761) % 97 }))
        .sort((a, b) => a.k - b.k).map((x) => x.o)
    : opts;
  return (
    <ol className="options" data-mode={q.scoring_mode}>
      {shown.map((o) => (
        <li key={o.id} className={o.correct ? "correct" : undefined} data-mode={q.scoring_mode}>
          <span className="opt-html" dangerouslySetInnerHTML={{ __html: o.html }} />
        </li>
      ))}
    </ol>
  );
}

export function MatchingView({ q }: { q: Question }) {
  // Pasangan benar (ids) dirender sebagai teks via matching_options.
  const rows = questionPairs(q);
  if (rows.length === 0) return <div className="matching"><p className="text-sm text-muted">Belum ada pasangan.</p></div>;
  return (
    <div className="matching">
      {rows.map((p, i) => (
        <div key={p.left + i} className="pair" data-correct={p.correct}>
          <span className="pair-html">{pairText(q, p.left)}</span> →{" "}
          <span className="pair-html">{pairText(q, p.right)}</span>
        </div>
      ))}
    </div>
  );
}

type Pair = { left: string; right: string; correct?: boolean };

function pairText(q: Question, id: string): string {
  const mo = q.matching_options;
  const all = [...(mo?.left ?? []), ...(mo?.right ?? [])];
  return all.find((x) => x.id === id)?.text ?? id;
}

// Pasangan menjodohkan disimpan di answer_key sebagai [{left,right}...].
export function questionPairs(q: Question): Pair[] {
  const key = q.answer_key as { pairs?: Pair[] } | null | undefined;
  return Array.isArray(key?.pairs) ? key!.pairs : [];
}

export function ScoringBadge({ q }: { q: Question }) {
  return (
    <span className="badge" data-mode={q.scoring_mode} title="mode penilaian">
      {q.scoring_mode === "partial" ? "parsial" : "semua-atau-tidak"}
      {q.scoring_mode === "partial" && q.partial_wrong_penalty ? " (–)" : ""}
    </span>
  );
}

export type { Option, Question };
