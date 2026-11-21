import XCTest
@testable import EnergyHub

final class Auth0ConfigurationTests: XCTestCase {
    func testLoadsRequiredAuth0Settings() throws {
        let configuration = try Auth0Configuration(values: [
            "Auth0Domain": "dev-tenant.us.auth0.com",
            "Auth0ClientId": "native-client-id",
            "Auth0Audience": "https://api.energyhub.kr4t3rdev.xyz",
        ])

        XCTAssertEqual(configuration.domain, "dev-tenant.us.auth0.com")
        XCTAssertEqual(configuration.clientId, "native-client-id")
        XCTAssertEqual(configuration.audience, "https://api.energyhub.kr4t3rdev.xyz")
    }

    func testRejectsMissingAuth0Settings() {
        XCTAssertThrowsError(try Auth0Configuration(values: [:]))
    }

    func testRejectsInsecureOrMalformedAuth0Endpoints() {
        XCTAssertThrowsError(try Auth0Configuration(values: [
            "Auth0Domain": "http://dev-tenant.us.auth0.com",
            "Auth0ClientId": "native-client-id",
            "Auth0Audience": "https://api.energyhub.kr4t3rdev.xyz",
        ]))
        XCTAssertThrowsError(try Auth0Configuration(values: [
            "Auth0Domain": "dev-tenant.us.auth0.com",
            "Auth0ClientId": "native-client-id",
            "Auth0Audience": "http://api.energyhub.kr4t3rdev.xyz",
        ]))
    }
}
