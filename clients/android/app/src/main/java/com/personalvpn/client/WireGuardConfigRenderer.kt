package com.personalvpn.client

import java.net.URI
import java.util.Base64

/** Converts the control-plane response to the official WireGuard config format. */
object WireGuardConfigRenderer {
    fun render(privateKey: String, configuration: TunnelConfiguration): String {
        require(isKey(privateKey) && isKey(configuration.serverPublicKey)) { "invalid WireGuard key" }
        require(configuration.addresses.isNotEmpty() && configuration.allowedIps.isNotEmpty()) { "configuration has no addresses/routes" }
        val endpoint = URI("udp://${configuration.endpoint}")
        require(endpoint.host != null && endpoint.port in 1..65535 && endpoint.userInfo == null && endpoint.path.isEmpty() && endpoint.query == null && endpoint.fragment == null) { "invalid WireGuard endpoint" }
        val builder = StringBuilder()
            .append("[Interface]\nPrivateKey = ").append(privateKey).append('\n')
            .append("Address = ").append(configuration.addresses.joinToString(", ")).append('\n')
            .append("DNS = ").append(configuration.dnsServers.joinToString(", ")).append('\n')
            .append("\n[Peer]\nPublicKey = ").append(configuration.serverPublicKey).append('\n')
            .append("Endpoint = ").append(configuration.endpoint).append('\n')
            .append("AllowedIPs = ").append(configuration.allowedIps.joinToString(", ")).append('\n')
            .append("PersistentKeepalive = ").append(configuration.persistentKeepaliveSeconds).append('\n')
        require(configuration.persistentKeepaliveSeconds in 0..120) { "invalid keepalive" }
        return builder.toString()
    }

    private fun isKey(value: String): Boolean = try {
        Base64.getDecoder().decode(value).size == 32
    } catch (_: IllegalArgumentException) { false }
}
