package com.personalvpn.client

import android.content.Context
import android.util.Base64
import kotlinx.serialization.json.Json

/** Stores the rotating token pair encrypted with a separate Android Keystore key. */
class AuthTokenStore(context: Context, private val secureStore: SecureKeyStore = SecureKeyStore(ALIAS)) {
    private val preferences = context.getSharedPreferences(PREFERENCES, Context.MODE_PRIVATE)
    private val json = Json { ignoreUnknownKeys = true }

    fun save(pair: TokenPair) {
        require(isSessionToken(pair.accessToken) && isSessionToken(pair.refreshToken)) { "token pair is invalid" }
        val encoded = Base64.encodeToString(secureStore.encrypt(json.encodeToString(TokenPair.serializer(), pair).toByteArray(Charsets.UTF_8)), Base64.NO_WRAP)
        check(preferences.edit().putString(TOKEN_PAIR, encoded).commit()) { "failed to persist auth tokens" }
    }

    fun load(): TokenPair? = preferences.getString(TOKEN_PAIR, null)?.let { encoded -> runCatching {
        val plaintext = secureStore.decrypt(Base64.decode(encoded, Base64.NO_WRAP)).toString(Charsets.UTF_8)
        this@AuthTokenStore.json.decodeFromString(TokenPair.serializer(), plaintext).also { pair ->
            require(isSessionToken(pair.accessToken) && isSessionToken(pair.refreshToken))
        }
    }.getOrNull() }

    fun clear() {
        check(preferences.edit().remove(TOKEN_PAIR).commit()) { "failed to remove auth tokens" }
        secureStore.delete()
    }

    private companion object {
        const val PREFERENCES = "personalvpn_auth_tokens"
        const val TOKEN_PAIR = "encrypted_token_pair"
        const val ALIAS = "personalvpn-auth"
        fun isSessionToken(value: String): Boolean = value.length == 47 && value.startsWith("pv1_")
    }
}
