package com.personalvpn.client

import android.Manifest
import android.app.Activity
import android.content.pm.PackageManager
import android.content.Intent
import android.graphics.Color
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.view.Gravity
import android.view.ViewGroup
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import android.text.InputType
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.serialization.json.Json
import javax.net.ssl.HttpsURLConnection

/** UI deliberately stays separate from tunnel installation and API state. */
class MainActivity : Activity() {
    private lateinit var status: TextView
    private val screenScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        val padding = (24 * resources.displayMetrics.density).toInt()
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            setPadding(padding, padding, padding, padding)
        }
        root.addView(TextView(this).apply {
            text = "PersonalVPN"
            textSize = 28f
            setTextColor(Color.BLACK)
            gravity = Gravity.CENTER
        }, matchContent())
        root.addView(TextView(this).apply {
            text = "Cliente Android / Android TV"
            textSize = 16f
            gravity = Gravity.CENTER
            setPadding(0, padding / 2, 0, padding)
        }, matchContent())
        status = TextView(this).apply {
            text = "Estado: pronto para configurar"
            textSize = 18f
            setPadding(0, padding, 0, padding)
        }
        root.addView(status, matchContent())
        val email = EditText(this).apply {
            hint = "Email"
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_EMAIL_ADDRESS
            setSingleLine(true)
        }
        root.addView(email, matchContent())
        val password = EditText(this).apply {
            hint = "Palavra-passe (mínimo 12 caracteres)"
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
            setSingleLine(true)
        }
        root.addView(password, matchContent())
        val debugTls = DebugTls.socketFactory(this)
        val controlPlaneNetwork = ControlPlaneNetwork.find(this)
        if (debugTls != null) HttpsURLConnection.setDefaultSSLSocketFactory(debugTls)
        val auth = VpnAuthClient(ApiEnvironment.baseUrl, sslSocketFactory = debugTls, controlPlaneNetwork = controlPlaneNetwork)
        val tokenStore = AuthTokenStore(this)
        var provisioning: VpnProvisioningCoordinator? = null
        lateinit var connect: Button
        suspend fun establishSession(tokens: TokenPair) {
            runCatching {
                tokenStore.save(tokens)
                val api = VpnApiClient(ApiEnvironment.baseUrl, tokens.accessToken, sslSocketFactory = debugTls, controlPlaneNetwork = controlPlaneNetwork)
                val currentProvisioning = VpnProvisioningCoordinator(this@MainActivity, api)
                val device = currentProvisioning.ensureDevice("Android local")
                val regions = api.listRegions()
                Triple(currentProvisioning, device, regions.count { it.available })
            }.onSuccess { (currentProvisioning, device, availableRegions) ->
                provisioning = currentProvisioning
                connect.isEnabled = true
                status.text = "Estado: dispositivo registado (${device.id}); regiões disponíveis: $availableRegions"
            }.onFailure {
                status.text = "Estado: sessão iniciada, mas o registo falhou (${it.message ?: "erro"})"
            }
        }
        connect = Button(this).apply {
            text = "Obter configuração e ligar"
            isEnabled = false
            setOnClickListener {
                val consent = VpnService.prepare(this@MainActivity)
                if (consent != null) {
                    status.text = "Estado: autoriza a ligação VPN e prime novamente"
                    startActivityForResult(consent, VPN_CONSENT_REQUEST)
                    return@setOnClickListener
                }
                status.text = "Estado: a pedir configuração…"
                screenScope.launch {
                    runCatching { provisioning?.requestConfiguration("pt-lis") ?: error("sessão não iniciada") }
                        .onSuccess { configuration ->
                            val intent = Intent(this@MainActivity, VpnTunnelService::class.java)
                                .setAction(VpnTunnelService.ACTION_START)
                                .putExtra(VpnTunnelService.EXTRA_CONFIGURATION, Json.encodeToString(configuration))
                            startForegroundService(intent)
                            status.text = "Estado: túnel ativo; pode testar a Internet"
                        }
                        .onFailure {
                            val message = it.message.orEmpty()
                            if (message.contains("API request failed: 401")) {
                                tokenStore.clear()
                                provisioning = null
                                connect.isEnabled = false
                                status.text = "Estado: sessão expirada; prime Entrar novamente"
                            } else {
                                status.text = "Estado: configuração indisponível (${message.ifBlank { "erro" }})"
                            }
                        }
                }
            }
        }
        root.addView(connect, matchContent())
        root.addView(Button(this).apply {
            text = "Criar conta e entrar"
            setOnClickListener {
                val address = email.text.toString().trim()
                val secret = password.text.toString()
                status.text = "Estado: a autenticar…"
                screenScope.launch {
                    runCatching {
                        auth.register(address, secret)
                        auth.login(address, secret)
                    }.onSuccess { establishSession(it) }
                        .onFailure { status.text = "Estado: falha de autenticação (${it.message ?: "erro"})" }
                }
            }
        }, matchContent())
        root.addView(Button(this).apply {
            text = "Entrar"
            setOnClickListener {
                val address = email.text.toString().trim()
                val secret = password.text.toString()
                status.text = "Estado: a iniciar sessão…"
                screenScope.launch {
                    runCatching { auth.login(address, secret) }
                        .onSuccess { establishSession(it) }
                        .onFailure { status.text = "Estado: falha de autenticação (${it.message ?: "erro"})" }
                }
            }
        }, matchContent())
        root.addView(Button(this).apply {
            text = "Terminar sessão"
            setOnClickListener {
                tokenStore.clear()
                provisioning = null
                connect.isEnabled = false
                startService(Intent(this@MainActivity, VpnTunnelService::class.java).setAction(VpnTunnelService.ACTION_STOP))
                status.text = "Estado: sessão terminada e túnel parado"
            }
        }, matchContent())
        root.addView(TextView(this).apply {
            text = "O motor WireGuard está integrado. Autoriza o serviço VPN e inicia sessão para obter uma configuração do servidor."
            textSize = 15f
            setTextColor(Color.DKGRAY)
            setPadding(0, 0, 0, padding)
        }, matchContent())
        root.addView(Button(this).apply {
            text = "Autorizar VPN"
            setOnClickListener { requestVpnConsent() }
        }, matchContent())
        setContentView(root)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), NOTIFICATION_PERMISSION_REQUEST)
        }
        tokenStore.load()?.let { savedTokens ->
            status.text = "Estado: a restaurar sessão…"
            screenScope.launch {
                runCatching { auth.refresh(savedTokens.refreshToken) }
                    .onSuccess { establishSession(it) }
                    .onFailure {
                        tokenStore.clear()
                        provisioning = null
                        connect.isEnabled = false
                        status.text = "Estado: sessão expirada; prime Entrar novamente"
                    }
            }
        }
    }

    override fun onDestroy() {
        screenScope.cancel()
        super.onDestroy()
    }

    private fun requestVpnConsent() {
        val intent = VpnService.prepare(this)
        if (intent == null) {
            status.text = "Estado: autorização VPN já concedida"
        } else {
            startActivityForResult(intent, VPN_CONSENT_REQUEST)
        }
    }

    @Deprecated("Use Activity Result APIs when the full UI flow is introduced")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == VPN_CONSENT_REQUEST) {
            status.text = if (resultCode == RESULT_OK) {
                "Estado: autorização VPN concedida"
            } else {
                "Estado: autorização VPN recusada"
            }
        }
    }

    private fun matchContent() = LinearLayout.LayoutParams(
        ViewGroup.LayoutParams.MATCH_PARENT,
        ViewGroup.LayoutParams.WRAP_CONTENT,
    )

    companion object {
        private const val NOTIFICATION_PERMISSION_REQUEST = 2001
        private const val VPN_CONSENT_REQUEST = 2002
    }
}
