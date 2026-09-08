import XCTest
@testable import PersonalVPNClient

final class WireGuardConfigRendererTests: XCTestCase {
    private let key = String(repeating: "A", count: 43) + "="

    func testRendersValidConfiguration() throws {
        let configuration = TunnelConfiguration(
            assignmentId: "assignment-1",
            endpoint: "vpn.example.test:51820",
            serverPublicKey: key,
            addresses: ["10.8.0.2/32"],
            dnsServers: ["10.8.0.1"],
            allowedIps: ["0.0.0.0/0", "::/0"],
            persistentKeepaliveSeconds: 25
        )

        let rendered = try WireGuardConfigRenderer.render(privateKey: key, configuration: configuration)
        XCTAssertTrue(rendered.contains("PrivateKey = \(key)"))
        XCTAssertTrue(rendered.contains("Endpoint = vpn.example.test:51820"))
    }

    func testRejectsEndpointWithQuery() {
        let configuration = TunnelConfiguration(
            assignmentId: "assignment-1",
            endpoint: "vpn.example.test:51820?redirect=1",
            serverPublicKey: key,
            addresses: ["10.8.0.2/32"],
            dnsServers: ["10.8.0.1"],
            allowedIps: ["0.0.0.0/0"],
            persistentKeepaliveSeconds: 0
        )

        XCTAssertThrowsError(try WireGuardConfigRenderer.render(privateKey: key, configuration: configuration))
    }
}
