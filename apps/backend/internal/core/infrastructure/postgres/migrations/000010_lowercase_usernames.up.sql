-- Usernames are now case-insensitive: stored and compared in lower case.
-- This backfills existing data so already-registered accounts keep working
-- (their JWTs are normalized to lower case on verify, so the subject must
-- match a lower-cased users.id).

-- Abort if two accounts differ only by case; collapsing them would merge
-- distinct users and corrupt their balances. These must be resolved by hand.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM users GROUP BY lower(id) HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'username case collision: two or more accounts differ only by case; resolve manually before applying this migration';
    END IF;
END $$;

-- The FKs that reference users(id) are not deferrable and have no ON UPDATE
-- CASCADE, so the parent key and its children cannot be re-cased while they are
-- enforced. Drop them, rewrite every user-id column, then recreate them exactly.
ALTER TABLE expenses          DROP CONSTRAINT expenses_payer_id_fkey;
ALTER TABLE splits            DROP CONSTRAINT splits_user_id_fkey;
ALTER TABLE group_members     DROP CONSTRAINT group_members_user_id_fkey;
ALTER TABLE group_invitations DROP CONSTRAINT group_invitations_inviter_id_fkey;
ALTER TABLE group_invitations DROP CONSTRAINT group_invitations_invitee_id_fkey;

UPDATE users             SET id          = lower(id)         WHERE id          <> lower(id);
UPDATE expenses          SET payer_id    = lower(payer_id)   WHERE payer_id    <> lower(payer_id);
UPDATE splits            SET user_id     = lower(user_id)    WHERE user_id     <> lower(user_id);
UPDATE group_members     SET user_id     = lower(user_id)    WHERE user_id     <> lower(user_id);
UPDATE group_invitations SET inviter_id  = lower(inviter_id) WHERE inviter_id  <> lower(inviter_id);
UPDATE group_invitations SET invitee_id  = lower(invitee_id) WHERE invitee_id  <> lower(invitee_id);
-- audit_logs.user_id is not FK-constrained but stores the actor's username; keep
-- it consistent so the activity feed resolves to the same accounts.
UPDATE audit_logs        SET user_id     = lower(user_id)    WHERE user_id     <> lower(user_id);

ALTER TABLE expenses          ADD CONSTRAINT expenses_payer_id_fkey            FOREIGN KEY (payer_id)    REFERENCES users(id);
ALTER TABLE splits            ADD CONSTRAINT splits_user_id_fkey               FOREIGN KEY (user_id)     REFERENCES users(id);
ALTER TABLE group_members     ADD CONSTRAINT group_members_user_id_fkey        FOREIGN KEY (user_id)     REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE group_invitations ADD CONSTRAINT group_invitations_inviter_id_fkey FOREIGN KEY (inviter_id)  REFERENCES users(id);
ALTER TABLE group_invitations ADD CONSTRAINT group_invitations_invitee_id_fkey FOREIGN KEY (invitee_id)  REFERENCES users(id);
