import SwiftUI

struct ContentView: View {
    let site: SiteOverview

    init(site: SiteOverview = exampleSite) {
        self.site = site
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                VStack(alignment: .leading, spacing: 8) {
                    Text("EnergyHub Universal")
                        .font(.title)
                        .fontWeight(.bold)
                    Text("MODO DEMO")
                        .font(.caption)
                        .fontWeight(.semibold)
                    Text("Una mirada más clara a tu energía.")
                        .font(.headline)
                }

                VStack(alignment: .leading, spacing: 8) {
                    Text("DATOS DE EJEMPLO")
                        .font(.caption)
                        .fontWeight(.bold)
                    Text("Estás explorando una demostración")
                        .fontWeight(.semibold)
                    Text("Todos los valores de esta pantalla son datos de ejemplo. No hay equipos conectados.")
                }
                .padding()
                .background(Color.secondary.opacity(0.15))
                .cornerRadius(12)

                VStack(alignment: .leading, spacing: 12) {
                    Text(site.name)
                        .font(.title2)
                        .fontWeight(.bold)
                    Text("Una vista general de cómo se organizará tu información energética.")
                    ForEach(site.readings, id: \.label) { reading in
                        VStack(alignment: .leading, spacing: 4) {
                            HStack {
                                Text(reading.label)
                                Spacer()
                                Text(reading.icon)
                            }
                            Text(formatReading(reading.value, unit: reading.unit))
                                .font(.title3)
                                .fontWeight(.semibold)
                            Text(reading.note)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                        .padding()
                        .background(Color.secondary.opacity(0.1))
                        .cornerRadius(12)
                    }
                }

                VStack(alignment: .leading, spacing: 12) {
                    Text("Dispositivos")
                        .font(.title2)
                        .fontWeight(.bold)
                    if hasDevices(site.devices) {
                        ForEach(site.devices, id: \.id) { device in
                            VStack(alignment: .leading, spacing: 4) {
                                Text(device.name)
                                    .fontWeight(.semibold)
                                Text(device.kind)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                Text(deviceStatusLabel(device))
                                Text(device.note)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                            .padding()
                            .background(Color.secondary.opacity(0.1))
                            .cornerRadius(12)
                        }
                    } else {
                        VStack(alignment: .leading, spacing: 8) {
                            Text("Aún no hay dispositivos")
                                .fontWeight(.semibold)
                            Text("Cuando la conexión esté disponible, aquí podrás consultar los equipos de tu instalación.")
                        }
                        .padding()
                        .background(Color.secondary.opacity(0.1))
                        .cornerRadius(12)
                    }
                    Text("La incorporación de equipos desde el móvil llegará en una fase posterior.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }

                Text("© EnergyHub Universal — PROYECTO EN DESARROLLO")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            .padding()
        }
    }
}
