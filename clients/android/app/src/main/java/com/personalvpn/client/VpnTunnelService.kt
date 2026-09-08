package com.personalvpn.client

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import com.wireguard.android.backend.GoBackend
import kotlinx.serialization.json.Json

/** Owns the OS VPN lifecycle; cryptographic tunnel work belongs to WireGuardEngine. */
class VpnTunnelService : GoBackend.VpnService() {
    private lateinit var engine: WireGuardEngine
    private var tunnelActive = false

    override fun onCreate() {
        super.onCreate()
        engine = GoBackendWireGuardEngine(this)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            engine.stop()
            tunnelActive = false
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
            return START_NOT_STICKY
        }
        if (tunnelActive) {
            updateNotification("VPN ativa")
            return START_STICKY
        }
        val notification = foregroundNotification("A iniciar VPN…")
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
        val encoded = intent?.getStringExtra(EXTRA_CONFIGURATION) ?: return START_NOT_STICKY
        val configuration = try { Json.decodeFromString(TunnelConfiguration.serializer(), encoded) } catch (_: Exception) { return START_NOT_STICKY }
        try {
            engine.start(configuration, this)
            tunnelActive = true
            updateNotification("VPN ativa")
        } catch (_: RuntimeException) {
            tunnelActive = false
            stopSelf()
        }
        return START_NOT_STICKY
    }

    override fun onDestroy() {
        engine.stop()
        tunnelActive = false
        stopForeground(STOP_FOREGROUND_REMOVE)
        super.onDestroy()
    }

    private fun updateNotification(text: String) {
        getSystemService(NotificationManager::class.java).notify(NOTIFICATION_ID, foregroundNotification(text))
    }

    private fun foregroundNotification(text: String): Notification {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(NotificationChannel(CHANNEL_ID, "VPN", NotificationManager.IMPORTANCE_LOW))
        }
        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("PersonalVPN")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.stat_sys_warning)
            .setOngoing(true)
            .build()
    }

    companion object {
        const val ACTION_START = "com.personalvpn.client.START"
        const val ACTION_STOP = "com.personalvpn.client.STOP"
        const val EXTRA_CONFIGURATION = "configuration"
        private const val CHANNEL_ID = "personalvpn-vpn"
        private const val NOTIFICATION_ID = 1001
    }
}
