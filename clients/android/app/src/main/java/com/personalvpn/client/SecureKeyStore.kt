package com.personalvpn.client

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey

/** Encrypts the WireGuard private key with a non-exportable Android Keystore key. */
class SecureKeyStore(private val alias: String = "personalvpn-wireguard") {
    init { require(alias.matches(Regex("[A-Za-z0-9._-]{1,64}"))) { "invalid keystore alias" } }

    private val store: KeyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }

    fun encryptPrivateKey(value: ByteArray): ByteArray {
        require(value.size == 32) { "WireGuard private key must be 32 bytes" }
        return encrypt(value)
    }

    fun encrypt(value: ByteArray): ByteArray {
        require(value.isNotEmpty()) { "value must not be empty" }
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, key())
        return cipher.iv + cipher.doFinal(value)
    }

    fun decryptPrivateKey(value: ByteArray): ByteArray {
        return decrypt(value).also { require(it.size == 32) { "decrypted WireGuard private key must be 32 bytes" } }
    }

    fun decrypt(value: ByteArray): ByteArray {
        require(value.size > 12) { "encrypted key is truncated" }
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, key(), javax.crypto.spec.GCMParameterSpec(128, value.copyOfRange(0, 12)))
        return cipher.doFinal(value.copyOfRange(12, value.size))
    }

    fun key(): SecretKey {
        if (!store.containsAlias(alias)) {
            KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").apply {
                init(KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM).setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE).build())
                generateKey()
            }
        }
        return (store.getEntry(alias, null) as KeyStore.SecretKeyEntry).secretKey
    }

    fun delete() {
        if (store.containsAlias(alias)) store.deleteEntry(alias)
    }
}
