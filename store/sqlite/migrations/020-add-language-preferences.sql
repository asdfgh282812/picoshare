-- preferred_language is the user's chosen interface language. NULL means
-- they haven't chosen one, so PicoShare falls back to their language cookie
-- or the site default.
ALTER TABLE users ADD COLUMN preferred_language TEXT CHECK (
    preferred_language IS NULL OR preferred_language IN ('en', 'zh-TW')
);

-- default_language is the administrator-configured fallback language for
-- visitors who haven't chosen one. NULL means PicoShare matches each
-- visitor's browser language instead of a fixed default.
ALTER TABLE settings ADD COLUMN default_language TEXT CHECK (
    default_language IS NULL OR default_language IN ('en', 'zh-TW')
);
