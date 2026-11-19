import Foundation

struct Reading {
    let label: String
    let value: Double?
    let unit: String
    let icon: String
    let note: String
    /// ISO 8601 del momento de la medida, o nil si no hay dato.
    let measuredAt: String?
}

struct DeviceSummary {
    let id: String
    let name: String
    let kind: String
    /// Estado del equipo, o nil si no hay dato.
    let status: String?
    /// Nota de presentación; en la fixture siempre es "Valor de ejemplo".
    let note: String
}

struct SiteOverview {
    let name: String
    /// ISO 8601 de la última actualización real, o nil si nunca hubo datos.
    let updatedAt: String?
    let readings: [Reading]
    let devices: [DeviceSummary]
}

// Valores sintéticos; esta fixture no es una integración de dispositivo ni de API.
// Mismos datos y textos que apps/web/app/site/example.ts.
let exampleSite = SiteOverview(
    name: "Casa de ejemplo",
    updatedAt: nil,
    readings: [
        Reading(label: "Potencia solar", value: 1.24, unit: "kW", icon: "☼", note: "Valor de ejemplo", measuredAt: nil),
        Reading(label: "Consumo", value: 0.86, unit: "kW", icon: "↗", note: "Valor de ejemplo", measuredAt: nil),
        Reading(label: "Batería", value: 72, unit: "%", icon: "◧", note: "Valor de ejemplo", measuredAt: nil),
        Reading(label: "Estado de red", value: nil, unit: "", icon: "⌁", note: "Sin datos disponibles", measuredAt: nil)
    ],
    devices: [
        DeviceSummary(id: "dev-inversor-1", name: "Inversor solar", kind: "inversor", status: "En línea", note: "Valor de ejemplo"),
        DeviceSummary(id: "dev-bateria-1", name: "Batería doméstica", kind: "batería", status: nil, note: "Valor de ejemplo")
    ]
)
