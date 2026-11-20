import Foundation

/**
 * Errores del cliente de API de EnergyHub Universal.
 */
enum ApiClientError: Error {
    case baseUrlNotConfigured
    case invalidResponse
    case requestFailed(Int, String)
    case decodingFailed(Error)
}

/**
 * Cliente HTTP mínimo para la API de EnergyHub Universal.
 *
 * La URL base se lee de la clave `APIBaseURL` en el `Info.plist` del bundle.
 * Cuando no está definida, el cliente opera en modo sin conexión.
 *
 * El token de autenticación (Bearer) se inyecta desde fuera. El cliente nunca
 * almacena ni lee tokens por su cuenta.
 */
final class ApiClient {
    private let session: URLSession
    private let baseUrl: String
    private let authToken: String?

    init(baseUrl: String, authToken: String? = nil, session: URLSession = .shared) {
        self.baseUrl = baseUrl.hasSuffix("/") ? String(baseUrl.dropLast()) : baseUrl
        self.authToken = authToken
        self.session = session
    }

    convenience init?(fromBundle bundle: Bundle = .main, authToken: String? = nil) {
        guard let baseUrl = bundle.object(forInfoDictionaryKey: "APIBaseURL") as? String,
              !baseUrl.isEmpty else {
            return nil
        }
        self.init(baseUrl: baseUrl, authToken: authToken)
    }

    /**
     * Ejecuta una petición GET con el token Bearer cuando esté disponible.
     */
    func get(path: String) async throws -> Data {
        guard var components = URLComponents(string: baseUrl) else {
            throw ApiClientError.baseUrlNotConfigured
        }
        components.path = (components.path ?? "").hasSuffix("/") ?
            (components.path! + path.dropFirst()) : (components.path! + path)

        guard let url = components.url else {
            throw ApiClientError.baseUrlNotConfigured
        }

        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if let token = authToken {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        let (data, response) = try await session.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse else {
            throw ApiClientError.invalidResponse
        }

        guard (200...299).contains(httpResponse.statusCode) else {
            let body = String(data: data, encoding: .utf8) ?? ""
            throw ApiClientError.requestFailed(httpResponse.statusCode, body)
        }

        return data
    }
}

/**
 * Lee la URL base de la API desde el Info.plist del bundle principal.
 * Devuelve nil cuando no está configurada.
 */
func readApiBaseUrl() -> String? {
    Bundle.main.object(forInfoDictionaryKey: "APIBaseURL") as? String
}
