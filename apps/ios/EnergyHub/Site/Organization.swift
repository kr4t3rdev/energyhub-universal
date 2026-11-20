import Foundation

/**
 * Organización del sujeto autenticado, tal como la devuelve
 * GET /v1/organizations (o /v1/me) del backend.
 */
struct Organization: Codable, Identifiable, Sendable {
    let id: String
    let name: String
    let role: String
}

/**
 * Respuesta actual del usuario autenticado.
 */
struct CurrentUser: Codable, Sendable {
    let subject: String
    let organizations: [Organization]
}

/**
 * Mapea los datos JSON del backend a [Organization], fallando cerrado cuando
 * faltan campos obligatorios.
 */
func mapCurrentUser(_ data: Data) throws -> CurrentUser {
    let decoder = JSONDecoder()
    do {
        return try decoder.decode(CurrentUser.self, from: data)
    } catch {
        throw ApiClientError.decodingFailed(error)
    }
}
