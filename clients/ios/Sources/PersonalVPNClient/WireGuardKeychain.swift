import Foundation
import Security

public enum WireGuardKeychainError: Error {
    case randomGenerationFailed(OSStatus)
    case keychainFailure(OSStatus)
    case invalidStoredKey
}

/// Stores one 32-byte WireGuard private key on this device only.
public final class WireGuardKeychain: @unchecked Sendable {
    private let account: String

    public init(account: String = "wireguard-private-key") {
        precondition(!account.isEmpty && account.count <= 128)
        self.account = account
    }

    public func loadOrCreate() throws -> Data {
        var query = baseQuery()
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne

        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)
        if status == errSecSuccess, let data = item as? Data {
            guard data.count == 32 else { throw WireGuardKeychainError.invalidStoredKey }
            return data
        }
        guard status == errSecItemNotFound else { throw WireGuardKeychainError.keychainFailure(status) }

        var bytes = [UInt8](repeating: 0, count: 32)
        let randomStatus = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        guard randomStatus == errSecSuccess else { throw WireGuardKeychainError.randomGenerationFailed(randomStatus) }
        let data = Data(bytes)

        var add = baseQuery()
        add[kSecValueData as String] = data
        add[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        let addStatus = SecItemAdd(add as CFDictionary, nil)
        if addStatus == errSecDuplicateItem { return try loadOrCreate() }
        guard addStatus == errSecSuccess else { throw WireGuardKeychainError.keychainFailure(addStatus) }
        return data
    }

    public func delete() throws {
        let status = SecItemDelete(baseQuery() as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw WireGuardKeychainError.keychainFailure(status)
        }
    }

    private func baseQuery() -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: "com.personalvpn",
            kSecAttrAccount as String: account
        ]
    }
}
