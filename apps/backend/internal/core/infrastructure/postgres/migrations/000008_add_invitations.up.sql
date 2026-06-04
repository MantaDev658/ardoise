CREATE TABLE group_invitations (
    id         UUID PRIMARY KEY,
    group_id   UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    inviter_id TEXT NOT NULL REFERENCES users(id),
    invitee_id TEXT NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX group_invitations_group_invitee ON group_invitations(group_id, invitee_id);
