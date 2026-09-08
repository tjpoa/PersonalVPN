package com.personalvpn.client

import org.junit.Assert.assertTrue
import org.junit.Test

class WireGuardConfigRendererTest {
    private val key = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

    @Test
    fun rendersValidatedUserspaceConfiguration() {
        val output = WireGuardConfigRenderer.render(
            key,
            TunnelConfiguration(
                assignmentId = "assignment-1",
                endpoint = "vpn.example.test:51820",
                serverPublicKey = key,
                addresses = listOf("10.0.0.2/32"),
                dnsServers = listOf("10.0.0.1"),
                allowedIps = listOf("0.0.0.0/0", "::/0"),
                persistentKeepaliveSeconds = 25,
            ),
        )

        assertTrue(output.contains("PrivateKey = $key"))
        assertTrue(output.contains("Endpoint = vpn.example.test:51820"))
        assertTrue(output.contains("AllowedIPs = 0.0.0.0/0, ::/0"))
    }

    @Test(expected = IllegalArgumentException::class)
    fun rejectsEndpointWithPath() {
        WireGuardConfigRenderer.render(
            key,
            TunnelConfiguration("assignment-1", "vpn.example.test:51820/path", key, listOf("10.0.0.2/32"), emptyList(), listOf("0.0.0.0/0"), 0),
        )
    }
}
