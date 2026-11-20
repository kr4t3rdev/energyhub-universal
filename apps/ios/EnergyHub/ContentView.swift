import SwiftUI

struct ContentView: View {
    let site: SiteOverview

    @State private var organizations: [Organization] = []
    @State private var isLoadingOrganizations = true
    @State private var organizationsError: String? = nil

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

                // Tu conexión a la API real
                VStack(alignment: .leading, spacing: 12) {
                    Text("Tus organizaciones")
                        .font(.title2)
                        .fontWeight(.bold)

                    if isLoadingOrganizations {
                        ProgressView("Cargando...")
                    } else if let error = organizationsError {
                        Text(error)
                            .font(.caption)
                            .foregroundStyle(.red)
                    } else if organizations.isEmpty {
                        if isApiConfigured() {
                            Text("No tienes organizaciones asignadas.")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        } else {
                            Text("La API no está configurada. Agrega APIBaseURL al Info.plist y obtén un token de autenticación para ver tus organizaciones reales.")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    } else {
                        ForEach(organizations, id: \.id) { org in
                            VStack(alignment: .leading, spacing: 4) {
                                Text(org.name)
                                    .fontWeight(.semibold)
                                Text(org.role)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                            .padding()
                            .background(Color.secondary.opacity(0.1))
                            .cornerRadius(12)
                        }
                    }
                }

                Text("© EnergyHub Universal — PROYECTO EN DESARROLLO")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            .padding()
        }
        .task {
            await loadOrganizations()
        }
    }

    private func loadOrganizations() async {
        isLoadingOrganizations = true
        organizationsError = nil

        do {
            let token = UserDefaults.standard.string(forKey: "energyhub_token")
            let provider = OrganizationApiProvider(authToken: token)
            let orgs = try await provider.getOrganizations()
            self.organizations = orgs
        } catch {
            if let apiError = error as? ApiClientError {
                self.organizationsError = "\(apiError)"
            } else {
                self.organizationsError = error.localizedDescription
            }
        }

        isLoadingOrganizations = false
    }
}
