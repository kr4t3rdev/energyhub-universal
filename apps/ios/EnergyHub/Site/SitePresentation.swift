import Foundation

/// Valor con unidad en español, o "—" si no hay dato. Nunca se infiere un valor.
func formatReading(_ value: Double?, unit: String) -> String {
    guard let value = value, value.isFinite else { return "—" }
    let formatter = NumberFormatter()
    formatter.locale = Locale(identifier: "es")
    formatter.maximumFractionDigits = 2
    guard let formatted = formatter.string(from: NSNumber(value: value)) else { return "—" }
    return unit.isEmpty ? formatted : "\(formatted) \(unit)"
}

/// Etiqueta de estado de un dispositivo, sin inventar datos.
func deviceStatusLabel(_ device: DeviceSummary) -> String {
    let status = device.status?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    return status.isEmpty ? "Sin datos disponibles" : status
}

/// Indica si hay al menos un dispositivo que mostrar.
func hasDevices(_ devices: [DeviceSummary]) -> Bool {
    return !devices.isEmpty
}
