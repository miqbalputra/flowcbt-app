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
  // Menjodohkan tampil tap-pilih di mobile & drag di editor (PRD 11);
  // di sini hanya menampilkan pasangan, interaksi di EditorPage.
  const rows = questionPairs(q);
  return (
    <div className="matching">
      {rows.map((p, i) => (
        <div key={i} className="pair" data-correct={p.correct}>
          <span className="pair-html" dangerouslySetInnerHTML={{ __html: p.left }} /> →{" "}
          <span className="pair-html" dangerouslySetInnerHTML={{ __html: p.right }} />
        </div>
      ))}
    </div>
  );
}

type Pair = { left: string; right: string; correct: boolean };

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
