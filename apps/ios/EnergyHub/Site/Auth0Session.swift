import Auth0
import Combine
import Foundation

@MainActor
final class Auth0Session: ObservableObject {
    @Published private(set) var accessToken: String?
    @Published private(set) var isLoading = false
    @Published var errorMessage: String?

    private let configuration: Auth0Configuration?
    private let credentialsManager: CredentialsManager?

    var isConfigured: Bool { configuration != nil }

    init(configuration: Auth0Configuration? = Auth0Configuration(bundle: .main)) {
        self.configuration = configuration
        if let configuration {
            self.credentialsManager = CredentialsManager(
                authentication: Auth0.authentication(
                    clientId: configuration.clientId,
                    domain: configuration.domain
                )
            )
        } else {
            self.credentialsManager = nil
        }
    }

    func restoreSession() async {
        guard let credentialsManager else { return }
        do {
            let credentials = try await credentialsManager.credentials()
            accessToken = credentials.accessToken
        } catch {
            accessToken = nil
        }
    }

    func login() async {
        guard let configuration, let credentialsManager else {
            errorMessage = "Falta configurar Auth0 en la app."
            return
        }

        isLoading = true
        errorMessage = nil
        defer { isLoading = false }

        do {
            let credentials = try await Auth0
                .webAuth(clientId: configuration.clientId, domain: configuration.domain)
                .audience(configuration.audience)
                .scope("openid profile email offline_access")
                .start()
            try credentialsManager.store(credentials: credentials)
            accessToken = credentials.accessToken
        } catch {
            errorMessage = "No se pudo iniciar sesión. Inténtalo de nuevo."
        }
    }

    func logout() async {
        guard let configuration else {
            accessToken = nil
            return
        }

        isLoading = true
        errorMessage = nil
        defer { isLoading = false }

        do {
            try await Auth0
                .webAuth(clientId: configuration.clientId, domain: configuration.domain)
                .logout()
        } catch {
            errorMessage = "Se cerró la sesión local, pero no se pudo cerrar la sesión web de Auth0."
        }

        do {
            try credentialsManager?.clear()
        } catch {
            errorMessage = "No se pudieron borrar las credenciales guardadas."
        }
        accessToken = nil
    }
}
