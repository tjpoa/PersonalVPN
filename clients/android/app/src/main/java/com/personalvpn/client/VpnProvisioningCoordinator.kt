package com.personalvpn.client

import android.content.Context
import java.util.UUID

/** Coordinates device identity and configuration without exposing private key material. */
class VpnProvisioningCoordinator(
    private val context: Context,
    private val api: VpnApiClient,
    private val identityStore: WireGuardIdentityStore = WireGuardIdentityStore(context.applicationContext),
) {
    private var device: DeviceResponse? = null

    suspend fun ensureDevice(name: String): DeviceResponse {
        require(name.isNotBlank() && name.length <= 80) { "device name is invalid" }
        val identity = identityStore.loadOrCreate()
        val platformRequest = RegisterDeviceRequest.forDevice(context, name, identity.publicKey)
        val existing = api.listDevices().firstOrNull {
            it.publicKey == identity.publicKey && it.platform == platformRequest.platform
        }
        return (existing ?: api.registerDevice(platformRequest)).also { device = it }
    }

    suspend fun requestConfiguration(regionId: String): TunnelConfiguration {
        val registered = device ?: error("device must be registered before requesting configuration")
        require(regionId.isNotBlank()) { "region is required" }
        // Include a monotonic component so emulator snapshots or an unusual
        // SecureRandom implementation cannot repeat a prior idempotency key.
        val idempotencyKey = "${UUID.randomUUID()}-${System.nanoTime()}"
        return api.requestConfiguration(registered.id, regionId, idempotencyKey)
    }
}
