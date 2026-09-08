package com.personalvpn.client

import android.content.Context
import android.net.VpnService
import com.wireguard.android.backend.GoBackend
import com.wireguard.android.backend.Tunnel
import com.wireguard.config.Config
import java.io.ByteArrayInputStream

/** Adapter boundary for the official wireguard-android/wireguard-go engine. */
interface WireGuardEngine {
    fun start(configuration: TunnelConfiguration, service: VpnService)
    fun stop()
}

class GoBackendWireGuardEngine(context: Context) : WireGuardEngine {
    private val backend = GoBackend(context.applicationContext)
    private val identityStore = WireGuardIdentityStore(context.applicationContext)
    private val tunnel = object : Tunnel {
        override fun getName(): String = TUNNEL_NAME
        override fun onStateChange(newState: Tunnel.State) = Unit
    }

    override fun start(configuration: TunnelConfiguration, service: VpnService) {
        check(GoBackend.VpnService.prepare(service) == null) { "VPN authorization is required" }
        val identity = identityStore.loadOrCreate()
        val rendered = WireGuardConfigRenderer.render(identity.privateKey, configuration)
        val config = ByteArrayInputStream(rendered.toByteArray(Charsets.UTF_8)).use { input ->
            Config.parse(input)
        }
        backend.setState(tunnel, Tunnel.State.UP, config)
    }

    override fun stop() {
        runCatching { backend.setState(tunnel, Tunnel.State.DOWN, null) }
    }

    companion object { private const val TUNNEL_NAME = "personalvpn" }
}
