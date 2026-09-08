package com.personalvpn.client

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class AuthTokenStoreTest {
    @Test
    fun roundTripAndClear() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        val store = AuthTokenStore(context, SecureKeyStore("test-auth-${System.currentTimeMillis()}"))
        val pair = TokenPair("pv1_" + "a".repeat(43), "pv1_" + "r".repeat(43), "2030-01-01T00:00:00Z", "2030-02-01T00:00:00Z")
        store.save(pair)
        assertEquals(pair, store.load())
        store.clear()
        assertNull(store.load())
    }
}
