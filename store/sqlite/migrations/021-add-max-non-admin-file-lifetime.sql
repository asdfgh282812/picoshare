-- max_non_admin_file_lifetime_days caps how long a non-admin user's own
-- uploads may live. NULL means no cap. The upper bound matches
-- picoshare.FileLifetimeInfinite (100 years).
ALTER TABLE settings ADD COLUMN max_non_admin_file_lifetime_days INTEGER CHECK (
    max_non_admin_file_lifetime_days IS NULL
    OR max_non_admin_file_lifetime_days BETWEEN 1 AND 36500
);
