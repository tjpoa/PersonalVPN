package com.personalvpn.client

import android.content.Context
import android.util.Base64
import com.wireguard.crypto.Key
import com.wireguard.crypto.KeyPair

/** Creates and persists one WireGuard identity per Android installation. */
class WireGuardIdentityStore(context: Context, private val secureKeyStore: SecureKeyStore = SecureKeyStore()) {
    private val preferences = context.getSharedPreferences(PREFERENCES, Context.MODE_PRIVATE)

    fun loadOrCreate(): WireGuardIdentity {
        val privateBytes = preferences.getString(PRIVATE_KEY, null)?.let { encoded ->
            secureKeyStore.decryptPrivateKey(Base64.decode(encoded, Base64.NO_WRAP))
        } ?: KeyPair().privateKey.bytes.also { generated ->
            check(preferences.edit()
                .putString(PRIVATE_KEY, Base64.encodeToString(secureKeyStore.encryptPrivateKey(generated), Base64.NO_WRAP))
                .commit()) { "failed to persist generated WireGuard identity" }
        }
        val pair = KeyPair(Key.fromBytes(privateBytes))
        return WireGuardIdentity(pair.privateKey.toBase64(), pair.publicKey.toBase64())
    }

    /** Removes the local identity during logout or device revocation. */
    fun delete() {
        check(preferences.edit().remove(PRIVATE_KEY).commit()) { "failed to remove stored WireGuard identity" }
        secureKeyStore.delete()
    }

    data class WireGuardIdentity(val privateKey: String, val publicKey: String)

    private companion object {
        const val PREFERENCES = "personalvpn_wireguard_identity"
        const val PRIVATE_KEY = "encrypted_private_key"
    }
}
