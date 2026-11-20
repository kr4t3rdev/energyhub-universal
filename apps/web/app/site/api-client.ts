/**
 * Cliente HTTP mínimo para la API de EnergyHub Universal.
 *
 * La URL base se lee de la variable de entorno NEXT_PUBLIC_API_BASE_URL. Cuando no
 * está definida, el cliente opera en modo sin conexión: todas las peticiones fallan
 * con un error claro en lugar de apuntar a una dirección por defecto que pudiera
 * filtrar datos o enviar peticiones a un host no previsto.
 *
 * El token de autenticación se inyecta desde fuera (authToken). El cliente nunca
 * almacena ni lee tokens por su cuenta: la capa de llamada es responsable de
 * obtenerlo del flujo OIDC y de pasarlo a cada petición.
 */
export interface ApiClientOptions {
  baseUrl: string
  authToken?: string
}

export class ApiClient {
  private readonly baseUrl: string
  private readonly authToken?: string

  constructor({ baseUrl, authToken }: ApiClientOptions) {
    this.baseUrl = baseUrl.replace(/\/$/, "")
    this.authToken = authToken
  }

  /**
   * Ejecuta una petición GET con el token Bearer cuando esté disponible.
   * Lanza un error si la respuesta no es exitosa.
   */
  async get(path: string, init?: RequestInit): Promise<Response> {
    const url = `${this.baseUrl}${path}`
    const headers: Record<string, string> = {
      Accept: "application/json",
      ...(init?.headers as Record<string, string> | undefined),
    }
    if (this.authToken) {
      headers.Authorization = `Bearer ${this.authToken}`
    }

    const response = await fetch(url, { ...init, headers })

    if (!response.ok) {
      const body = await response.text().catch(() => "")
      throw new Error(
        `API request failed: ${response.status} ${response.statusText}${body ? ` · ${body}` : ""}`,
      )
    }

    return response
  }
}

/**
 * Lee la URL base de la API desde el entorno de Next.js. Cuando no está
 * definida, devuelve una cadena vacía para que el llamante pueda decidir
 * quedarse en modo demo.
 */
export function readApiBaseUrl(): string {
  return typeof process !== "undefined" && process.env.NEXT_PUBLIC_API_BASE_URL
    ? process.env.NEXT_PUBLIC_API_BASE_URL
    : ""
}
