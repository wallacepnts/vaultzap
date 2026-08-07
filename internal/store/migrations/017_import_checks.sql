-- Coherence findings about the imported file, as JSON: [{"code":"out_of_order","count":1}].
-- Separate from warnings_text, which is the parser saying what it could not read; these are
-- about the file's own shape and are translated in the UI, so only codes are stored.
ALTER TABLE imports ADD COLUMN checks_text TEXT;
