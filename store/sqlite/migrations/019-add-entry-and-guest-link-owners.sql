-- owner_user_id is NULL only for entries and guest links created before
-- PicoShare supported multiple users. RecordUserLogin claims these rows for
-- the first administrator as soon as one logs in.
ALTER TABLE entries ADD COLUMN owner_user_id INTEGER REFERENCES users (id);

ALTER TABLE guest_links ADD COLUMN owner_user_id INTEGER REFERENCES users (id);

CREATE INDEX idx_entries_owner_user_id ON entries (owner_user_id);

CREATE INDEX idx_guest_links_owner_user_id ON guest_links (owner_user_id);
