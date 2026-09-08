package com.personalvpn.client

import android.net.Network
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.json.Json
import java.net.HttpURLConnection
import java.net.URI
import java.net.URL
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLSocketFactory

@Serializable
data class AuthCredentials(val email: String, val password: String)

@Serializable
data class RegistrationResponse(val userId: String)

@Serializable
data class TokenPair(
    val accessToken: String,
    val refreshToken: String,
    val accessExpiresAt: String,
    val refreshExpiresAt: String,
)

/** Unauthenticated account operations; token persistence belongs to a platform secure store. */
class VpnAuthClient(private val baseUrl: String, private val json: Json = Json { ignoreUnknownKeys = true }, private val sslSocketFactory: SSLSocketFactory? = null, private val controlPlaneNetwork: Network? = null) {
    private val apiUri = runCatching { URI(baseUrl) }.getOrElse { throw IllegalArgumentException("API URL is invalid", it) }

    init {
        require(apiUri.scheme == "https" && !apiUri.host.isNullOrBlank() && apiUri.userInfo == null && apiUri.query == null && apiUri.fragment == null) {
            "API must use an HTTPS origin without embedded credentials"
        }
    }

    suspend fun register(email: String, password: String): RegistrationResponse = withContext(Dispatchers.IO) {
        validateCredentials(email, password)
        json.decodeFromString(RegistrationResponse.serializer(), post("/v1/auth/register", json.encodeToString(AuthCredentials.serializer(), AuthCredentials(email, password))))
    }

    suspend fun login(email: String, password: String): TokenPair = withContext(Dispatchers.IO) {
        validateCredentials(email, password)
        json.decodeFromString(TokenPair.serializer(), post("/v1/auth/login", json.encodeToString(AuthCredentials.serializer(), AuthCredentials(email, password))))
    }

    suspend fun refresh(refreshToken: String): TokenPair = withContext(Dispatchers.IO) {
        require(refreshToken.length == 47 && refreshToken.startsWith("pv1_")) { "refresh token is invalid" }
        val body = "{\"refreshToken\":${json.encodeToString(String.serializer(), refreshToken)}}"
        json.decodeFromString(TokenPair.serializer(), post("/v1/auth/refresh", body))
    }

    private fun validateCredentials(email: String, password: String) {
        require(email.isNotBlank() && email.length <= 320 && email.contains('@')) { "email is invalid" }
        require(password.length in 12..1024) { "password length is invalid" }
    }

    private fun post(path: String, body: String): String {
        val connection = (openConnection(path) as HttpURLConnection).apply {
            if (this is HttpsURLConnection && sslSocketFactory != null) this.sslSocketFactory = sslSocketFactory
            requestMethod = "POST"; doOutput = true; connectTimeout = 10_000; readTimeout = 15_000; instanceFollowRedirects = false
            setRequestProperty("Content-Type", "application/json"); setRequestProperty("Accept", "application/json")
        }
        try {
            connection.outputStream.use { it.write(body.toByteArray(Charsets.UTF_8)) }
            val status = connection.responseCode
            val stream = if (status in 200..299) connection.inputStream else connection.errorStream
                ?: throw IllegalStateException("API returned no response body")
            val response = stream.bufferedReader().use { it.readText() }
            check(status in 200..299) { "API request failed: $status" }
            return response
        } finally { connection.disconnect() }
    }

    private fun openConnection(path: String) =
        (controlPlaneNetwork?.openConnection(URL(baseUrl.trimEnd('/') + path))
            ?: URL(baseUrl.trimEnd('/') + path).openConnection())
}
