package com.energyhub.universal.site

import java.text.NumberFormat
import java.util.Locale

/** Valor con unidad en espanol, o "—" si no hay dato. Nunca se infiere un valor. */
fun formatReading(value: Double?, unit: String): String {
    if (value == null || !value.isFinite()) return "\u2014"
    val formatted = NumberFormat.getInstance(Locale("es")).apply {
        maximumFractionDigits = 2
    }.format(value)
    return if (unit.isNotEmpty()) "$formatted $unit" else formatted
}

/** Etiqueta de estado de un dispositivo, sin inventar datos. */
fun deviceStatusLabel(device: DeviceSummary): String {
    val status = device.status?.trim()
    return if (status.isNullOrEmpty()) "Sin datos disponibles" else status
}

/** Indica si hay al menos un dispositivo que mostrar. */
fun hasDevices(devices: List<DeviceSummary>): Boolean = devices.isNotEmpty()
