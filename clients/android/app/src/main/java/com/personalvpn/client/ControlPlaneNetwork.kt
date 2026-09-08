package com.personalvpn.client

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities

/**
 * Picks the physical network for control-plane calls. A full-tunnel VPN must
 * not route the request that obtains its own configuration back through TUN.
 */
object ControlPlaneNetwork {
    fun find(context: Context): Network? {
        val connectivity = context.getSystemService(ConnectivityManager::class.java)
        val active = connectivity.activeNetwork
        if (active != null && isUsable(connectivity, active)) return active
        return connectivity.allNetworks.firstOrNull { isUsable(connectivity, it) }
    }

    private fun isUsable(connectivity: ConnectivityManager, network: Network): Boolean {
        val capabilities = connectivity.getNetworkCapabilities(network) ?: return false
        return !capabilities.hasTransport(NetworkCapabilities.TRANSPORT_VPN) &&
            capabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
    }
}
