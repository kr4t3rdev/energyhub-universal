-- Roll back the membership database hardening of 000004.
--
-- The result is the 000003 state: the admin group can read the tenant tables
-- again, the functions check energyhub_membership_admin and the audit trail
-- is gone. The ownership of the tables and of the app schema stays with
-- energyhub_owner on purpose: giving it back to an application role would
-- re-open the write path that 000004 closed.

BEGIN;

-- Restore the 000003 functions --------------------------------------------------

CREATE OR REPLACE FUNCTION app.create_organization(p_organization_id text, p_name text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
    IF NOT pg_has_role(session_user, 'energyhub_membership_admin', 'member') THEN
        RAISE EXCEPTION 'caller is not a membership administrator' USING ERRCODE = '42501';
    END IF;
    IF p_organization_id IS NULL OR btrim(p_organization_id) = '' THEN
        RAISE EXCEPTION 'organization_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_name IS NULL OR btrim(p_name) = '' THEN
        RAISE EXCEPTION 'organization name is required' USING ERRCODE = '22023';
    END IF;

    INSERT INTO public.organizations (id, name)
    VALUES (btrim(p_organization_id), btrim(p_name))
    ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;
END;
$$;

CREATE OR REPLACE FUNCTION app.grant_membership(p_organization_id text, p_subject text, p_role text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
    IF NOT pg_has_role(session_user, 'energyhub_membership_admin', 'member') THEN
        RAISE EXCEPTION 'caller is not a membership administrator' USING ERRCODE = '42501';
    END IF;
    IF p_organization_id IS NULL OR btrim(p_organization_id) = '' THEN
        RAISE EXCEPTION 'organization_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_subject IS NULL OR btrim(p_subject) = '' THEN
        RAISE EXCEPTION 'subject is required' USING ERRCODE = '22023';
    END IF;
    IF p_role IS NULL OR p_role NOT IN ('owner', 'admin', 'member', 'viewer') THEN
        RAISE EXCEPTION 'invalid membership role' USING ERRCODE = '22023';
    END IF;

    INSERT INTO public.organization_memberships (organization_id, subject, role)
    VALUES (btrim(p_organization_id), btrim(p_subject), p_role)
    ON CONFLICT (organization_id, subject) DO UPDATE SET role = EXCLUDED.role;
END;
$$;

CREATE OR REPLACE FUNCTION app.revoke_membership(p_organization_id text, p_subject text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
    IF NOT pg_has_role(session_user, 'energyhub_membership_admin', 'member') THEN
        RAISE EXCEPTION 'caller is not a membership administrator' USING ERRCODE = '42501';
    END IF;
    IF p_organization_id IS NULL OR btrim(p_organization_id) = '' THEN
        RAISE EXCEPTION 'organization_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_subject IS NULL OR btrim(p_subject) = '' THEN
        RAISE EXCEPTION 'subject is required' USING ERRCODE = '22023';
    END IF;

    DELETE FROM public.organization_memberships
    WHERE organization_id = btrim(p_organization_id) AND subject = btrim(p_subject);
END;
$$;

-- The functions keep their owner even when the rollback runs with a
-- privileged role that recreates them.
ALTER FUNCTION app.create_organization(text, text) OWNER TO energyhub_owner;
ALTER FUNCTION app.grant_membership(text, text, text) OWNER TO energyhub_owner;
ALTER FUNCTION app.revoke_membership(text, text) OWNER TO energyhub_owner;

-- Restore the 000003 policies ----------------------------------------------------

DROP POLICY IF EXISTS organizations_select_member ON public.organizations;
CREATE POLICY organizations_select_member ON public.organizations
    FOR SELECT
    USING (
        pg_has_role(current_user, 'energyhub_membership_admin', 'member')
        OR EXISTS (
            SELECT 1
            FROM public.organization_memberships AS membership
            WHERE membership.organization_id = public.organizations.id
              AND membership.subject = current_setting('app.current_subject', true)
        )
    );

DROP POLICY IF EXISTS memberships_select_own ON public.organization_memberships;
CREATE POLICY memberships_select_own ON public.organization_memberships
    FOR SELECT
    USING (
        subject = current_setting('app.current_subject', true)
        OR pg_has_role(current_user, 'energyhub_membership_admin', 'member')
    );

-- Restore the 000003 privileges ---------------------------------------------------

GRANT SELECT ON public.organizations, public.organization_memberships TO energyhub_membership_admin;

-- Drop the audit trail -----------------------------------------------------------

DROP TABLE IF EXISTS app.admin_audit_log;

COMMIT;
