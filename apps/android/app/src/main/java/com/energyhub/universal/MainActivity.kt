package com.energyhub.universal

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.energyhub.universal.site.deviceStatusLabel
import com.energyhub.universal.site.exampleSite
import com.energyhub.universal.site.formatReading
import com.energyhub.universal.site.hasDevices

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MaterialTheme {
                val site = exampleSite
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState())
                        .padding(16.dp),
                ) {
                    Text("EnergyHub Universal", style = MaterialTheme.typography.headlineMedium)
                    Text("MODO DEMO", style = MaterialTheme.typography.labelSmall)
                    Spacer(modifier = Modifier.height(16.dp))
                    Text("DATOS DE EJEMPLO", style = MaterialTheme.typography.labelMedium)
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
                    Text("PROYECTO EN DESARROLLO", style = MaterialTheme.typography.labelSmall)
                }
            }
        }
    }
}
