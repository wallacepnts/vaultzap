-- The login screen asks for a username too, chosen by the user on first run. A row whose
-- username is empty counts as "setup not finished": that is what the previous version's
-- generated-password row becomes, so it lands on the setup screen instead of a login form
-- it could never satisfy.
ALTER TABLE credentials ADD COLUMN username TEXT NOT NULL DEFAULT '';
