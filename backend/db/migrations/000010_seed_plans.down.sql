DELETE FROM platform_settings WHERE key IN (
  'retention.readonly_days',
  'retention.snapshot_days',
  'retention.audit_months',
  'limits.max_students_per_tenant',
  'storage.default_quota_bytes'
);
DELETE FROM plans WHERE code IN (
  'free_trial','per_exam_s','per_exam_m','per_exam_l','monthly_m','monthly_l'
);
