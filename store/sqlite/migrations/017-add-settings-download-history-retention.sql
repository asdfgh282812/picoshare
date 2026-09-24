-- download_history_retention_days is NULL when PicoShare keeps download
-- history forever. The upper bound matches
-- picoshare.MaxDownloadHistoryRetentionDays.
ALTER TABLE settings ADD COLUMN download_history_retention_days INTEGER CHECK (
    download_history_retention_days IS NULL
    OR download_history_retention_days BETWEEN 1 AND 3650
);
