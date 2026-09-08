import Foundation
import Security

public enum AuthTokenKeychainError: Error {
    case keychainFailure(OSStatus)
    case invalidToken
}

public final class AuthTokenKeychain: @unchecked Sendable {
    private let account: String

    public init(account: String = "auth-token") {
        precondition(!account.isEmpty && account.count <= 128)
        self.account = account
    }

    public func save(_ tokens: TokenPair) throws {
        guard Self.isSessionToken(tokens.accessToken), Self.isSessionToken(tokens.refreshToken) else {
            throw AuthTokenKeychainError.invalidToken
        }
        let data = try JSONEncoder().encode(tokens)
        var query = baseQuery()
        query[kSecValueData as String] = data
        query[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        let status = SecItemAdd(query as CFDictionary, nil)
        if status == errSecDuplicateItem {
            let update = SecItemUpdate(baseQuery() as CFDictionary, [kSecValueData as String: data] as CFDictionary)
            guard update == errSecSuccess else { throw AuthTokenKeychainError.keychainFailure(update) }
        } else if status != errSecSuccess {
            throw AuthTokenKeychainError.keychainFailure(status)
        }
    }

    public func load() throws -> TokenPair? {
        var query = baseQuery()
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess, let data = item as? Data else { throw AuthTokenKeychainError.keychainFailure(status) }
        let tokens = try JSONDecoder().decode(TokenPair.self, from: data)
        guard Self.isSessionToken(tokens.accessToken), Self.isSessionToken(tokens.refreshToken) else {
            throw AuthTokenKeychainError.invalidToken
        }
        return tokens
    }

    public func delete() throws {
        let status = SecItemDelete(baseQuery() as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else { throw AuthTokenKeychainError.keychainFailure(status) }
    }

    private func baseQuery() -> [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: "com.personalvpn", kSecAttrAccount as String: account]
    }

    private static func isSessionToken(_ value: String) -> Bool { value.count == 47 && value.hasPrefix("pv1_") }
}
