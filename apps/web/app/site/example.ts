import type { SiteOverview } from "./types";

// Valores sintéticos; esta fixture no es una integración de dispositivo ni de API.
export const exampleSite: SiteOverview = {
  name: "Casa de ejemplo",
  updatedAt: null,
  readings: [
    { label: "Potencia solar", value: 1.24, unit: "kW", icon: "☼", note: "Valor de ejemplo", measuredAt: null },
    { label: "Consumo", value: 0.86, unit: "kW", icon: "↗", note: "Valor de ejemplo", measuredAt: null },
    { label: "Batería", value: 72, unit: "%", icon: "◧", note: "Valor de ejemplo", measuredAt: null },
    { label: "Estado de red", value: null, unit: "", icon: "⌁", note: "Sin datos disponibles", measuredAt: null },
  ],
  devices: [
    { id: "dev-inversor-1", name: "Inversor solar", kind: "inversor", status: "En línea", note: "Valor de ejemplo" },
    { id: "dev-bateria-1", name: "Batería doméstica", kind: "batería", status: null, note: "Valor de ejemplo" },
  ],
};
