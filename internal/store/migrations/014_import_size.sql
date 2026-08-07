-- Size of the imported unit, recorded at import time: the file may later be moved to
-- .imported/ or deleted, and stat-ing at render time would show nothing.
ALTER TABLE imports ADD COLUMN size_bytes INTEGER;
