package com.energyhub.universal

import android.content.Context
import android.content.SharedPreferences
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.energyhub.universal.site.ApiClientError
import com.energyhub.universal.site.Organization
import com.energyhub.universal.site.OrganizationApiProvider
import com.energyhub.universal.site.deviceStatusLabel
import com.energyhub.universal.site.exampleSite
import com.energyhub.universal.site.formatReading
import com.energyhub.universal.site.hasDevices
import com.energyhub.universal.site.isApiConfigured

class MainActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            val context = this@MainActivity
            MaterialTheme {
                val site = exampleSite
                val token = rememberSaveable { mutableStateOf<String?>(null) }
                val organizationsState = remember { mutableStateOf<List<Organization>>(emptyList()) }
                val isLoadingState = remember { mutableStateOf(true) }
                val errorState = remember { mutableStateOf<String?>(null) }

                LaunchedEffect(Unit) {
                    isLoadingState.value = true
                    errorState.value = null
                    try {
                        val prefs: SharedPreferences = context.getSharedPreferences(
                            "energyhub_prefs",
                            Context.MODE_PRIVATE,
                        )
                        val storedToken = prefs.getString("energyhub_token", null)
                        val provider = OrganizationApiProvider(storedToken)
                        val orgs = provider.getOrganizations()
                        organizationsState.value = orgs
                    } catch (e: Exception) {
                        errorState.value = when (e) {
                            is ApiClientError.RequestFailed -> "Error ${e.statusCode}: ${e.body}"
                            is ApiClientError.BaseUrlNotConfigured ->
                                "La API no est\u00e1 configurada. Define API_BASE_URL en build.gradle.kts."
                            is ApiClientError.DecodingFailed ->
                                "Error al leer la respuesta: ${e.source.message ?: e.message}"
                            else -> e.message ?: "Error desconocido"
                        }
                    } finally {
                        isLoadingState.value = false
                    }
                }

                val organizations = organizationsState.value
                val isLoading = isLoadingState.value
                val error = errorState.value

                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState())
                        .padding(16.dp),
                ) {
                    Text("EnergyHub Universal", style = MaterialTheme.typography.headlineMedium)
                    Text("MODO DEMO", style = MaterialTheme.typography.labelSmall)
                    Spacer(modifier = Modifier.height(16.dp))
                    Text(
                        "Est\u00e1s explorando una demostraci\u00f3n. " +
                            "Todos los valores de esta pantalla son datos de ejemplo. " +
                            "No hay equipos conectados.",
                    )
                    Spacer(modifier = Modifier.height(16.dp))
                    Text(site.name, style = MaterialTheme.typography.titleLarge)
                    Text(site.updatedAt ?: "Sin datos disponibles")
                    Spacer(modifier = Modifier.height(16.dp))
                    Text("Lecturas", style = MaterialTheme.typography.titleMedium)
                    for (reading in site.readings) {
                        Text("${reading.label}: ${formatReading(reading.value, reading.unit)}")
                        Text(reading.note, style = MaterialTheme.typography.bodySmall)
                    }
                    Spacer(modifier = Modifier.height(16.dp))
                    Text("Dispositivos (${site.devices.size})", style = MaterialTheme.typography.titleMedium)
                    if (hasDevices(site.devices)) {
                        for (device in site.devices) {
                            Text("${device.name} (${device.kind}): ${deviceStatusLabel(device)}")
                            Text(device.note, style = MaterialTheme.typography.bodySmall)
                        }
                    } else {
                        Text("A\u00fan no hay dispositivos")
                        Text(
                            "Cuando la conexi\u00f3n est\u00e9 disponible, aqu\u00ed podr\u00e1s " +
                                "consultar los equipos de tu instalaci\u00f3n.",
                        )
                    }
                    Spacer(modifier = Modifier.height(16.dp))

                    // Tu conexión a la API real
                    Text("Tus organizaciones", style = MaterialTheme.typography.titleMedium)
                    if (isLoading) {
                        Text("Cargando...", style = MaterialTheme.typography.bodySmall)
                    } else if (error != null) {
                        Text(error!!, style = MaterialTheme.typography.bodySmall.copy(
                            color = MaterialTheme.colorScheme.error,
                        ))
                    } else if (!isApiConfigured()) {
                        Text(
                            "La API no est\u00e1 configurada. Define API_BASE_URL en build.gradle.kts " +
                                "y obt\u00e9n un token de autenticación para ver tus organizaciones reales.",
                            style = MaterialTheme.typography.bodySmall,
                        )
                    } else if (organizations.isEmpty()) {
                        Text("No tienes organizaciones asignadas.", style = MaterialTheme.typography.bodySmall)
                    } else {
                        for (org in organizations) {
                            Text(org.name, style = MaterialTheme.typography.bodyMedium)
                            Text(
                                org.role,
                                style = MaterialTheme.typography.bodySmall.copy(
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                ),
                            )
                        }
                    }

                    Spacer(modifier = Modifier.height(16.dp))
                    Text("PROYECTO EN DESARROLLO", style = MaterialTheme.typography.labelSmall)
                }
            }
        }
    }
}
