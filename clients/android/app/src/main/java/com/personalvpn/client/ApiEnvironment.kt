package com.personalvpn.client

/** Build-time API origin; debug points at the local emulator proxy only. */
object ApiEnvironment {
    const val baseUrl: String = BuildConfig.API_BASE_URL
}
