import Foundation

struct Auth0Configuration {
    let domain: String
    let clientId: String
    let audience: String

    init(values: [String: Any]) throws {
        guard let clientId = Self.nonEmptyString(values["Auth0ClientId"]),
              let audience = Self.nonEmptyString(values["Auth0Audience"]),
              Self.isHTTPSURL(audience),
              let rawDomain = Self.nonEmptyString(values["Auth0Domain"]),
              let domain = Self.normalizedHTTPSDomain(rawDomain) else {
            throw Auth0ConfigurationError.invalidConfiguration
        }

        self.domain = domain
        self.clientId = clientId
        self.audience = audience
    }

    init?(bundle: Bundle = .main) {
        do {
            try self.init(values: bundle.infoDictionary ?? [:])
        } catch {
            return nil
        }
    }

    private static func nonEmptyString(_ value: Any?) -> String? {
        guard let value = value as? String else { return nil }
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    private static func normalizedHTTPSDomain(_ value: String) -> String? {
        let urlString = value.contains("://") ? value : "https://\(value)"
        guard let components = URLComponents(string: urlString),
              components.scheme == "https",
              let host = components.host,
              !host.isEmpty,
              components.user == nil,
              components.password == nil,
              components.query == nil,
              components.fragment == nil,
              components.path.isEmpty || components.path == "/" else {
            return nil
        }
        return host
    }

    private static func isHTTPSURL(_ value: String) -> Bool {
        guard let components = URLComponents(string: value) else { return false }
        return components.scheme == "https" && components.host != nil
    }
}

enum Auth0ConfigurationError: Error {
    case invalidConfiguration
}
