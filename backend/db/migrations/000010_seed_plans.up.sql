-- 000010: seed paket (PRD 3.1) & pengaturan awal. Semua dapat diubah Super Admin tanpa deploy.
INSERT INTO plans (code, name, type, price_idr, student_min, student_max, exam_quota, duration_days, sort_order) VALUES
  ('free_trial',   'Gratis / Trial',        'free',     0,     1,  10,   1,  14, 10),
  ('per_exam_s',   'Per Ujian — Kecil',     'per_exam', 10000, 11, 40,   3,  NULL, 20),
  ('per_exam_m',   'Per Ujian — Menengah',  'per_exam', 20000, 41, 70,   7,  NULL, 21),
  ('per_exam_l',   'Per Ujian — Besar',     'per_exam', 30000, 71, 100,  12, NULL, 22),
  ('monthly_m',    'Bulanan — Menengah',    'monthly',  50000, 1,  50,   NULL, 30, 30),
  ('monthly_l',    'Bulanan — Besar',       'monthly',  70000, 51, 100,  NULL, 30, 31)
ON CONFLICT (code) DO NOTHING;

INSERT INTO platform_settings (key, value) VALUES
  ('retention.readonly_days',        '7'),
  ('retention.snapshot_days',        '30'),
  ('retention.audit_months',         '12'),
  ('limits.max_students_per_tenant', '100'),
  ('storage.default_quota_bytes',    '1073741824')
ON CONFLICT (key) DO NOTHING;
