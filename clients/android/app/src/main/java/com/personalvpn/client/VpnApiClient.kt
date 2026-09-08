package com.personalvpn.client

import android.content.Context
import android.content.res.Configuration
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.net.HttpURLConnection
import android.net.Network
import java.net.URI
import java.net.URL
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLSocketFactory
import java.util.UUID

@Serializable
data class RegisterDeviceRequest(val name: String, val platform: String, val publicKey: String) {
    companion object {
        fun forDevice(context: Context, name: String, publicKey: String): RegisterDeviceRequest {
            val mode = context.getSystemService(Context.UI_MODE_SERVICE) as android.app.UiModeManager
            val platform = if (mode.currentModeType == Configuration.UI_MODE_TYPE_TELEVISION) "android_tv" else "android"
            return RegisterDeviceRequest(name, platform, publicKey)
        }
    }
}

@Serializable
data class DeviceResponse(val id: String, val name: String, val platform: String, val publicKey: String, val state: String)

@Serializable
data class RegionResponse(val id: String, val displayName: String, val available: Boolean)

@Serializable
data class ConfigurationRequest(val regionId: String)

@Serializable
data class TunnelConfiguration(val assignmentId: String, val endpoint: String, val serverPublicKey: String, val addresses: List<String>, val dnsServers: List<String>, val allowedIps: List<String>, val persistentKeepaliveSeconds: Int)

class VpnApiClient(private val baseUrl: String, private val accessToken: String, private val json: Json = Json { ignoreUnknownKeys = true }, private val sslSocketFactory: SSLSocketFactory? = null, private val controlPlaneNetwork: Network? = null) {
    private val apiUri = runCatching { URI(baseUrl) }.getOrElse { throw IllegalArgumentException("API URL is invalid", it) }
    init {
        require(apiUri.scheme == "https" && !apiUri.host.isNullOrBlank() && apiUri.userInfo == null && apiUri.query == null && apiUri.fragment == null) {
            "API must use an HTTPS origin without embedded credentials"
        }
        require(accessToken.isNotBlank()) { "API access token is required" }
    }

    suspend fun registerDevice(request: RegisterDeviceRequest): DeviceResponse = withContext(Dispatchers.IO) {
        json.decodeFromString(DeviceResponse.serializer(), post("/v1/devices", json.encodeToString(RegisterDeviceRequest.serializer(), request)))
    }
    suspend fun listDevices(): List<DeviceResponse> = withContext(Dispatchers.IO) {
        json.decodeFromString(ListSerializer(DeviceResponse.serializer()), get("/v1/devices"))
    }
    suspend fun listRegions(): List<RegionResponse> = withContext(Dispatchers.IO) {
        json.decodeFromString(ListSerializer(RegionResponse.serializer()), get("/v1/regions"))
    }
    suspend fun requestConfiguration(deviceId: String, regionId: String, idempotencyKey: String): TunnelConfiguration = withContext(Dispatchers.IO) {
        val path = "/v1/devices/${UUID.fromString(deviceId)}/configuration"
        val body = json.encodeToString(ConfigurationRequest.serializer(), ConfigurationRequest(regionId.also { require(it.isNotBlank()) }))
        val key = idempotencyKey.also { require(it.isNotBlank() && it.length in 16..128) }
        val response = try {
            post(path, body, key)
        } catch (error: IllegalStateException) {
            // A 409 here means this key is already associated with a different
            // request; retry once with a fresh key rather than surfacing a
            // misleading configuration error to the user.
            if (!error.message.orEmpty().startsWith("API request failed: 409")) throw error
            post(path, body, UUID.randomUUID().toString())
        }
        json.decodeFromString(TunnelConfiguration.serializer(), response)
    }

    private fun post(path: String, body: String, idempotencyKey: String? = null): String {
        val connection = (openConnection(path) as HttpURLConnection).apply {
            if (this is HttpsURLConnection && sslSocketFactory != null) this.sslSocketFactory = sslSocketFactory
            requestMethod = "POST"; doOutput = true; connectTimeout = 10_000; readTimeout = 15_000
            instanceFollowRedirects = false
            setRequestProperty("Authorization", "Bearer $accessToken"); setRequestProperty("Content-Type", "application/json")
            idempotencyKey?.let { setRequestProperty("Idempotency-Key", it) }
        }
        try {
            connection.outputStream.use { it.write(body.toByteArray(Charsets.UTF_8)) }
            val status = connection.responseCode
            val stream = if (status in 200..299) connection.inputStream else connection.errorStream
                ?: throw IllegalStateException("API returned no response body")
            val response = stream.bufferedReader().use { it.readText() }
            check(status in 200..299) {
                val detail = response.replace(Regex("\\s+"), " ").trim().take(240)
                "API request failed: $status${if (detail.isNotEmpty()) " ($detail)" else ""}"
            }
            return response
        } finally {
            connection.disconnect()
        }
    }

    private fun get(path: String): String {
        val connection = (openConnection(path) as HttpURLConnection).apply {
            if (this is HttpsURLConnection && sslSocketFactory != null) this.sslSocketFactory = sslSocketFactory
            requestMethod = "GET"; connectTimeout = 10_000; readTimeout = 15_000
            instanceFollowRedirects = false
            setRequestProperty("Authorization", "Bearer $accessToken")
            setRequestProperty("Accept", "application/json")
        }
        try {
            val status = connection.responseCode
            val stream = if (status in 200..299) connection.inputStream else connection.errorStream
                ?: throw IllegalStateException("API returned no response body")
            val response = stream.bufferedReader().use { it.readText() }
            check(status in 200..299) {
                val detail = response.replace(Regex("\\s+"), " ").trim().take(240)
                "API request failed: $status${if (detail.isNotEmpty()) " ($detail)" else ""}"
            }
            return response
        } finally {
            connection.disconnect()
        }
    }

    private fun openConnection(path: String) =
        (controlPlaneNetwork?.openConnection(URL(baseUrl.trimEnd('/') + path))
            ?: URL(baseUrl.trimEnd('/') + path).openConnection())
}
