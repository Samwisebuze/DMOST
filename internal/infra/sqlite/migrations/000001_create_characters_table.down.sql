-- Up never reads this file. It exists so that migrate Down and Drop work, and
-- so a reader can see what 000001 did without reconstructing it from the up.
DROP TABLE characters;
