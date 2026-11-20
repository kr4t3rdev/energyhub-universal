import type { OrganizationRole } from "./types"

/**
 * Organización del sujeto autenticado, tal como la devuelve
 * GET /v1/organizations (o /v1/me) del backend.
 */
export interface Organization {
  id: string
  name: string
  role: OrganizationRole
}

/**
 * Respuesta actual del usuario autenticado.
 */
export interface CurrentUser {
  subject: string
  organizations: Organization[]
}

/**
 * Mapea el objeto JSON del backend a Organization[], ignorando campos
 * desconocidos y fallando cerrado cuando faltan campos obligatorios.
 */
export function mapCurrentUser(json: unknown): CurrentUser {
  if (!json || typeof json !== "object") {
    throw new Error("invalid CurrentUser response: expected an object")
  }

  const record = json as Record<string, unknown>
  const subject = record["subject"]
  if (typeof subject !== "string" || subject === "") {
    throw new Error("invalid CurrentUser response: missing subject")
  }

  const organizationsRaw = record["organizations"]
  if (!Array.isArray(organizationsRaw)) {
    throw new Error("invalid CurrentUser response: organizations is not an array")
  }

  const organizations: Organization[] = []
  for (const entry of organizationsRaw) {
    if (!entry || typeof entry !== "object") {
      throw new Error("invalid CurrentUser response: organization entry is not an object")
    }
    const org = entry as Record<string, unknown>
    const id = org["id"]
    const name = org["name"]
    const role = org["role"]
    if (typeof id !== "string" || id === "") {
      throw new Error("invalid CurrentUser response: organization missing id")
    }
    if (typeof name !== "string" || name === "") {
      throw new Error("invalid CurrentUser response: organization missing name")
    }
    if (
      typeof role !== "string" ||
      (role !== "owner" && role !== "admin" && role !== "member" && role !== "viewer")
    ) {
      throw new Error("invalid CurrentUser response: organization has invalid role")
    }
    organizations.push({ id, name, role: role as OrganizationRole })
  }

  return { subject, organizations }
}
