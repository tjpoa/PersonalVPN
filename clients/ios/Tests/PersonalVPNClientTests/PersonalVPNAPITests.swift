import Foundation
import XCTest
@testable import PersonalVPNClient

final class PersonalVPNAPITests: XCTestCase {
    func testListRegionsSendsBearerToken() async throws {
        let token = "pv1_" + String(repeating: "a", count: 43)
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [RecordingURLProtocol.self]
        let session = URLSession(configuration: configuration)
        let api = PersonalVPNAPI(baseURL: URL(string: "https://vpn.test")!, accessToken: token, session: session)

        _ = try await api.listRegions()

        XCTAssertEqual(RecordingURLProtocol.lastRequest?.value(forHTTPHeaderField: "Authorization"), "Bearer \(token)")
    }
}

private final class RecordingURLProtocol: URLProtocol {
    static var lastRequest: URLRequest?

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        Self.lastRequest = request
        let response = HTTPURLResponse(url: request.url!, statusCode: 200, httpVersion: nil, headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: Data("[]".utf8))
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}
