import Foundation

private final class NoRedirectDelegate: NSObject, URLSessionTaskDelegate {
    func urlSession(_ session: URLSession, task: URLSessionTask,
                    willPerformHTTPRedirection response: HTTPURLResponse,
                    newRequest request: URLRequest,
                    completionHandler: @escaping (URLRequest?) -> Void) {
        completionHandler(nil)
    }
}

private func secureSession(_ supplied: URLSession?) -> URLSession {
    supplied ?? URLSession(configuration: .ephemeral, delegate: NoRedirectDelegate(), delegateQueue: nil)
}

public struct RegisterDeviceRequest: Codable, Sendable {
    public let name: String
    public let platform: String
    public let publicKey: String

    public init(name: String, publicKey: String) {
        precondition(!name.isEmpty && name.count <= 80)
        self.name = name
        self.platform = "ios"
        self.publicKey = publicKey
    }
}

public struct DeviceResponse: Codable, Sendable {
    public let id: UUID
    public let name: String
    public let platform: String
    public let publicKey: String
    public let state: String
}

public struct TunnelConfiguration: Codable, Sendable {
    public let assignmentId: String
    public let endpoint: String
    public let serverPublicKey: String
    public let addresses: [String]
    public let dnsServers: [String]
    public let allowedIps: [String]
    public let persistentKeepaliveSeconds: Int
}

public struct AuthCredentials: Codable, Sendable {
    public let email: String
    public let password: String
    public init(email: String, password: String) {
        precondition(email.contains("@") && email.count <= 320)
        precondition((12...1024).contains(password.count))
        self.email = email; self.password = password
    }
}

public struct TokenPair: Codable, Sendable {
    public let accessToken: String
    public let refreshToken: String
    public let accessExpiresAt: String
    public let refreshExpiresAt: String
}

public struct RegionResponse: Codable, Sendable {
    public let id: String
    public let displayName: String
    public let available: Bool
}

public struct PersonalVPNAuth: Sendable {
    public let baseURL: URL
    private let session: URLSession
    public init(baseURL: URL, session: URLSession? = nil) {
        precondition(baseURL.scheme == "https" && baseURL.host != nil && baseURL.user == nil && baseURL.query == nil && baseURL.fragment == nil)
        self.baseURL = baseURL; self.session = secureSession(session)
    }
    public func register(email: String, password: String) async throws -> UUID {
        let data = try await post(path: "/v1/auth/register", body: AuthCredentials(email: email, password: password))
        struct Response: Codable { let userId: UUID }
        return try JSONDecoder().decode(Response.self, from: data).userId
    }
    public func login(email: String, password: String) async throws -> TokenPair {
        try await JSONDecoder().decode(TokenPair.self, from: post(path: "/v1/auth/login", body: AuthCredentials(email: email, password: password)))
    }
    private func post<T: Encodable>(path: String, body: T) async throws -> Data {
        var request = URLRequest(url: baseURL.appendingPathComponent(String(path.dropFirst())))
        request.httpMethod = "POST"; request.timeoutInterval = 15
        request.httpBody = try JSONEncoder().encode(body); request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse, http.url == request.url, http.url?.scheme == "https", (200..<300).contains(http.statusCode) else { throw URLError(.badServerResponse) }
        return data
    }
}

public protocol WireGuardExtension: Sendable {
    func start(configuration: TunnelConfiguration) async throws
    func stop() async
}

public struct PersonalVPNAPI: Sendable {
    public let baseURL: URL
    public let accessToken: String
    private let session: URLSession

    public init(baseURL: URL, accessToken: String, session: URLSession? = nil) {
        precondition(baseURL.scheme == "https" && baseURL.host != nil && baseURL.user == nil &&
                     baseURL.query == nil && baseURL.fragment == nil,
                     "PersonalVPN API requires an HTTPS origin without embedded credentials")
        precondition(Self.isSessionToken(accessToken), "PersonalVPN API requires a valid session token")
        self.baseURL = baseURL
        self.accessToken = accessToken
        self.session = secureSession(session)
    }

    public func register(_ request: RegisterDeviceRequest) async throws -> DeviceResponse {
        let data = try await post(path: "/v1/devices", body: request, idempotencyKey: nil)
        return try JSONDecoder().decode(DeviceResponse.self, from: data)
    }

    public func listDevices() async throws -> [DeviceResponse] {
        try await get(path: "/v1/devices", as: [DeviceResponse].self)
    }

    public func listRegions() async throws -> [RegionResponse] {
        try await get(path: "/v1/regions", as: [RegionResponse].self)
    }

    public func requestConfiguration(deviceID: UUID, regionID: String, idempotencyKey: String) async throws -> TunnelConfiguration {
        guard !regionID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
              !idempotencyKey.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
              (16...128).contains(idempotencyKey.count) else { throw URLError(.badURL) }
        let data = try await post(path: "/v1/devices/\(deviceID.uuidString)/configuration", body: ConfigurationRequest(regionId: regionID), idempotencyKey: idempotencyKey)
        return try JSONDecoder().decode(TunnelConfiguration.self, from: data)
    }

    private func post<T: Encodable>(path: String, body: T, idempotencyKey: String?) async throws -> Data {
        var request = URLRequest(url: baseURL.appendingPathComponent(String(path.dropFirst())))
        request.httpMethod = "POST"
        request.timeoutInterval = 15
        request.httpBody = try JSONEncoder().encode(body)
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        if let idempotencyKey { precondition((16...128).contains(idempotencyKey.count)); request.setValue(idempotencyKey, forHTTPHeaderField: "Idempotency-Key") }
        let (data, response) = try await session.data(for: request)
        // URLSession follows redirects by default; reject a changed final URL so
        // credentials are never accepted from a different origin or path.
        guard let http = response as? HTTPURLResponse,
              http.url == request.url,
              http.url?.scheme == "https",
              (200..<300).contains(http.statusCode) else { throw URLError(.badServerResponse) }
        return data
    }

    private func get<T: Decodable>(path: String, as type: T.Type) async throws -> T {
        var request = URLRequest(url: baseURL.appendingPathComponent(String(path.dropFirst())))
        request.httpMethod = "GET"; request.timeoutInterval = 15; request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse, http.url == request.url, http.url?.scheme == "https", (200..<300).contains(http.statusCode) else { throw URLError(.badServerResponse) }
        return try JSONDecoder().decode(type, from: data)
    }

    private static func isSessionToken(_ value: String) -> Bool {
        value.count == 47 && value.hasPrefix("pv1_")
    }
}

private struct ConfigurationRequest: Codable, Sendable {
    let regionId: String
}
