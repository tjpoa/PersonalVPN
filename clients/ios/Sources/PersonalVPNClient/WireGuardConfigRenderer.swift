import Foundation

public enum WireGuardConfigError: Error {
    case invalidKey
    case invalidEndpoint
    case invalidConfiguration
}

public enum WireGuardConfigRenderer {
    public static func render(privateKey: String, configuration: TunnelConfiguration) throws -> String {
        guard isKey(privateKey), isKey(configuration.serverPublicKey) else { throw WireGuardConfigError.invalidKey }
        guard !configuration.addresses.isEmpty, !configuration.allowedIps.isEmpty,
              configuration.persistentKeepaliveSeconds >= 0,
              configuration.persistentKeepaliveSeconds <= 120 else { throw WireGuardConfigError.invalidConfiguration }
        guard let components = URLComponents(string: "udp://\(configuration.endpoint)"),
              components.host != nil, (1...65535).contains(components.port ?? 0),
              components.user == nil, components.path.isEmpty, components.query == nil,
              components.fragment == nil else { throw WireGuardConfigError.invalidEndpoint }
        return """
        [Interface]
        PrivateKey = \(privateKey)
        Address = \(configuration.addresses.joined(separator: ", "))
        DNS = \(configuration.dnsServers.joined(separator: ", "))

        [Peer]
        PublicKey = \(configuration.serverPublicKey)
        Endpoint = \(configuration.endpoint)
        AllowedIPs = \(configuration.allowedIps.joined(separator: ", "))
        PersistentKeepalive = \(configuration.persistentKeepaliveSeconds)
        """
    }

    private static func isKey(_ value: String) -> Bool {
        guard let data = Data(base64Encoded: value) else { return false }
        return data.count == 32
    }
}
