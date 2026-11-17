CREATE TABLE organizations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE organization_memberships (
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    subject TEXT NOT NULL CHECK (length(btrim(subject)) > 0),
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, subject)
);

CREATE INDEX organization_memberships_subject_idx
    ON organization_memberships (subject, organization_id);
