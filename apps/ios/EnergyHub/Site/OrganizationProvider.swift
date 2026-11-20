import Foundation

/**
 * Proveedor de organizaciones que llama a la API real.
 *
 * Cuando la URL base de la API no está configurada, devuelve un arreglo vacío
 * para que la UI pueda decidir quedarse en modo demo.
 */
final class OrganizationApiProvider {
    private let client: ApiClient?

    init(authToken: String? = nil) {
        self.client = ApiClient(fromBundle: .main, authToken: authToken)
    }

    func getOrganizations() async throws -> [Organization] {
        guard let client = client else {
            return []
        }
        let data = try await client.get(path: "/v1/organizations")
        let currentUser = try mapCurrentUser(data)
        return currentUser.organizations
    }
}

/**
 * Indica si la aplicación está configurada para hablar con la API real.
 */
func isApiConfigured() -> Bool {
    readApiBaseurl() != nil
}

private func readApiBaseurl() -> String? {
    Bundle.main.object(forInfoDictionaryKey: "APIBaseURL") as? String
}
