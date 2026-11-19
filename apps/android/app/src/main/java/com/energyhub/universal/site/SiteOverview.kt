package com.energyhub.universal.site

/** Lectura de una instalacion; value null significa sin datos, nunca cero. */
data class Reading(
    val label: String,
    val value: Double?,
    val unit: String,
    val icon: String,
    val note: String,
    /** ISO 8601 del momento de la medida, o null si no hay dato. */
    val measuredAt: String?,
)

data class DeviceSummary(
    val id: String,
    val name: String,
    val kind: String,
    /** Estado del equipo, o null si no hay dato. */
    val status: String?,
    /** Nota de presentacion; en la fixture siempre es "Valor de ejemplo". */
    val note: String,
)

data class SiteOverview(
    val name: String,
    /** ISO 8601 de la ultima actualizacion real, o null si nunca hubo datos. */
    val updatedAt: String?,
    val readings: List<Reading>,
    val devices: List<DeviceSummary>,
)

// Valores sinteticos; esta fixture no es una integracion de dispositivo ni de API.
// Mismos datos y textos que apps/web/app/site/example.ts.
val exampleSite: SiteOverview = SiteOverview(
    name = "Casa de ejemplo",
    updatedAt = null,
    readings = listOf(
        Reading("Potencia solar", 1.24, "kW", "\u2600", "Valor de ejemplo", null),
        Reading("Consumo", 0.86, "kW", "\u2197", "Valor de ejemplo", null),
        Reading("Bater\u00eda", 72.0, "%", "\u25e7", "Valor de ejemplo", null),
        Reading("Estado de red", null, "", "\u2301", "Sin datos disponibles", null),
    ),
    devices = listOf(
        DeviceSummary("dev-inversor-1", "Inversor solar", "inversor", "En l\u00ednea", "Valor de ejemplo"),
        DeviceSummary("dev-bateria-1", "Bater\u00eda dom\u00e9stica", "bater\u00eda", null, "Valor de ejemplo"),
    ),
)
