-- "Oldest pin" has to mean pin order, not the message's own date: ordering by sent_at
-- would make pinning an old message a trap, first out on the very next pin. This column
-- also orders the strip under the header.
ALTER TABLE messages ADD COLUMN pinned_at TEXT;

-- Backfills rows pinned before this migration, so they have an order at all.
UPDATE messages SET pinned_at = sent_at WHERE pinned = 1 AND pinned_at IS NULL;
