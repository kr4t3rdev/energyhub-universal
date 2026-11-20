'use client'

import { useEffect, useState } from "react"
import { OrganizationApiProvider, isApiConfigured } from "./organization-provider"
import type { Organization } from "./organization"

/**
 * Client Component que lee el token de autenticación de localStorage (clave
 * `energyhub_token`, establecido por el flujo OIDC del usuario) y muestra las
 * organizaciones del sujeto autenticado consultando GET /v1/organizations.
 *
 * Cuando la API no está configurada o no hay token, se muestra un mensaje
 * informativo en lugar de datos inventados.
 */
export default function OrganizationsList() {
  const [organizations, setOrganizations] = useState<Organization[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    async function load() {
      try {
        setLoading(true)
        setError(null)

        if (!isApiConfigured()) {
          setOrganizations([])
          return
        }

        const token =
          typeof window !== "undefined" && window.localStorage
            ? window.localStorage.getItem("energyhub_token")
            : null

        const provider = new OrganizationApiProvider(token || undefined)
        const orgs = await provider.getOrganizations()

        if (!cancelled) {
          setOrganizations(orgs)
        }
      } catch (err) {
        if (!cancelled) {
          setError(
            err instanceof Error
              ? err.message
              : "no se pudieron cargar las organizaciones",
          )
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }

    load()
    return () => {
      cancelled = true
    }
  }, [])

  if (loading) {
    return (
      <section className="organizations-panel">
        <h2>Tus organizaciones</h2>
        <p>Cargando...</p>
      </section>
    )
  }

  if (error) {
    return (
      <section className="organizations-panel">
        <h2>Tus organizaciones</h2>
        <p style={{ color: "#dc2626" }}>{error}</p>
      </section>
    )
  }

  if (!isApiConfigured()) {
    return (
      <section className="organizations-panel">
        <h2>Tus organizaciones</h2>
        <p>
          La API no está configurada. Define NEXT_PUBLIC_API_BASE_URL y obtén un
          token de autenticación para ver tus organizaciones reales.
        </p>
      </section>
    )
  }

  if (organizations.length === 0) {
    return (
      <section className="organizations-panel">
        <h2>Tus organizaciones</h2>
        <p>No tienes organizaciones asignadas.</p>
      </section>
    )
  }

  return (
    <section className="organizations-panel">
      <h2>Tus organizaciones</h2>
      <ul className="organization-list">
        {organizations.map((org) => (
          <li key={org.id} className="organization-item">
            <div className="organization-name">{org.name}</div>
            <div className="organization-role">{org.role}</div>
          </li>
        ))}
      </ul>
    </section>
  )
}
