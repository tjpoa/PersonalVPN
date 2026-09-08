package com.personalvpn.client

import kotlinx.coroutines.runBlocking
import org.junit.Test

class VpnAuthClientTest {
    @Test(expected = IllegalArgumentException::class)
    fun rejectsHttpEndpoint() {
        VpnAuthClient("http://api.example.test")
    }

    @Test(expected = IllegalArgumentException::class)
    fun rejectsShortPasswordBeforeNetwork() {
        runBlocking {
        VpnAuthClient("https://api.example.test").login("user@example.test", "too-short")
        }
    }

    @Test(expected = IllegalArgumentException::class)
    fun rejectsMalformedEmailBeforeNetwork() {
        runBlocking {
        VpnAuthClient("https://api.example.test").register("not-an-email", "long-enough-password")
        }
    }
}
