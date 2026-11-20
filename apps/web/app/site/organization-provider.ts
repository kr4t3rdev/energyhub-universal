import type { Organization } from "./organization"
import { mapCurrentUser } from "./organization"
import { ApiClient, readApiBaseUrl } from "./api-client"

/**
 * Proveedor de organizaciones que llama a la API real.
 *
 * Cuando la URL base de la API no está configurada, devuelve un arreglo vacío
 * para que la UI pueda decidir quedarse en modo demo.
 */
export class OrganizationApiProvider {
  private readonly client: ApiClient | null

  constructor(authToken?: string) {
    const baseUrl = readApiBaseUrl()
    this.client = baseUrl ? new ApiClient({ baseUrl, authToken }) : null
  }

  async getOrganizations(): Promise<Organization[]> {
    if (!this.client) {
      return []
    }

    const response = await this.client.get("/v1/organizations")
    const json = await response.json()
    const currentUser = mapCurrentUser(json)
    return currentUser.organizations
  }
}

/**
 * Indica si la aplicación está configurada para hablar con la API real.
 */
export function isApiConfigured(): boolean {
  return readApiBaseUrl().length > 0
}
