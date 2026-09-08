package com.personalvpn.client

import org.junit.Test

class VpnApiClientTest {
    @Test(expected = IllegalArgumentException::class)
    fun rejectsHttpApi() {
        VpnApiClient("http://api.example.test", "token")
    }

    @Test(expected = IllegalArgumentException::class)
    fun rejectsApiWithEmbeddedCredentials() {
        VpnApiClient("https://user:password@api.example.test", "token")
    }

    @Test(expected = IllegalArgumentException::class)
    fun rejectsApiWithQuery() {
        VpnApiClient("https://api.example.test?debug=true", "token")
    }

    @Test(expected = IllegalArgumentException::class)
    fun rejectsBlankToken() {
        VpnApiClient("https://api.example.test", " ")
    }
}
