package com.personalvpn.client

import android.content.Context
import java.io.InputStream
import java.security.KeyStore
import java.security.cert.CertificateFactory
import javax.net.ssl.SSLContext
import javax.net.ssl.SSLSocketFactory
import javax.net.ssl.TrustManagerFactory

object DebugTls {
    fun socketFactory(context: Context): SSLSocketFactory? {
        if (!BuildConfig.DEBUG) return null
        // Local development certificates are optional and never committed.
        val rootId = context.resources.getIdentifier("caddy_local_root", "raw", context.packageName)
        val intermediateId = context.resources.getIdentifier("caddy_local_intermediate", "raw", context.packageName)
        if (rootId == 0 && intermediateId == 0) return null
        check(rootId != 0 && intermediateId != 0) { "Both local CA certificates are required" }
        val certificateFactory = CertificateFactory.getInstance("X.509")
        fun readCertificate(resourceId: Int) = context.resources.openRawResource(resourceId).use(InputStream::readBytes).let {
            certificateFactory.generateCertificate(it.inputStream())
        }
        val root = readCertificate(rootId)
        val intermediate = readCertificate(intermediateId)
        val keyStore = KeyStore.getInstance(KeyStore.getDefaultType()).apply {
            load(null, null)
            setCertificateEntry("caddy-local-root", root)
            setCertificateEntry("caddy-local-intermediate", intermediate)
        }
        val trust = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm()).apply { init(keyStore) }
        return SSLContext.getInstance("TLS").apply { init(null, trust.trustManagers, null) }.socketFactory
    }
}
