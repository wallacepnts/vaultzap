-- The parser's warning text, so /imports can show which line failed.
ALTER TABLE imports ADD COLUMN warnings_text TEXT;
