import { describe, expect, it } from "vitest";
import {
  deviceStatusLabel,
  formatReading,
  freshnessLabel,
  hasDevices,
  readingFreshness,
} from "./presentation";
import type { DeviceSummary } from "./types";

describe("formatReading", () => {
  it("formatea el valor con su unidad en español", () => {
    expect(formatReading(1.24, "kW")).toBe("1,24 kW");
    expect(formatReading(72, "%")).toBe("72 %");
  });

  it("muestra un guion para valores faltantes sin inventar datos", () => {
    expect(formatReading(null, "kW")).toBe("—");
    expect(formatReading(Number.NaN, "kW")).toBe("—");
  });

  it("no añade separador cuando la lectura no lleva unidad", () => {
    expect(formatReading(5, "")).toBe("5");
  });
});

describe("readingFreshness", () => {
  const now = new Date("2026-11-16T12:00:00Z");

  it("marca ausencia cuando no hay fecha o no es válida", () => {
    expect(readingFreshness(null, now)).toBe("missing");
    expect(readingFreshness("no-es-fecha", now)).toBe("missing");
  });

  it("marca fresca dentro del umbral", () => {
    expect(readingFreshness("2026-11-16T11:45:00Z", now)).toBe("fresh");
  });

  it("marca obsoleta fuera del umbral", () => {
    expect(readingFreshness("2026-11-16T10:00:00Z", now)).toBe("stale");
  });

  it("tolera relojes ligeramente adelantados", () => {
    expect(readingFreshness("2026-11-16T12:01:00Z", now)).toBe("fresh");
  });

  it("usa el umbral personalizado", () => {
    expect(readingFreshness("2026-11-16T11:00:00Z", now, 90)).toBe("fresh");
    expect(readingFreshness("2026-11-16T11:00:00Z", now, 30)).toBe("stale");
  });
});

describe("freshnessLabel", () => {
  const now = new Date("2026-11-16T12:00:00Z");

  it("explica la ausencia de datos sin inventarlos", () => {
    expect(freshnessLabel(null, now)).toBe("Sin datos disponibles");
  });

  it("describe una lectura reciente", () => {
    expect(freshnessLabel("2026-11-16T11:55:00Z", now)).toBe("Actualizado hace 5 min");
  });

  it("usa horas cuando la antigüedad supera una hora", () => {
    expect(freshnessLabel("2026-11-16T09:30:00Z", now, 180)).toBe("Actualizado hace 2 h");
  });

  it("avisa cuando la lectura está desactualizada", () => {
    expect(freshnessLabel("2026-11-16T09:00:00Z", now)).toBe("Datos desactualizados · hace 3 h");
  });
});

describe("deviceStatusLabel", () => {
  it("muestra el estado presente del dispositivo", () => {
    const device: DeviceSummary = {
      id: "dev-inversor-1",
      name: "Inversor solar",
      kind: "inversor",
      status: "En línea",
      note: "Valor de ejemplo",
    };
    expect(deviceStatusLabel(device)).toBe("En línea");
  });

  it("no inventa estado cuando es null", () => {
    const device: DeviceSummary = {
      id: "dev-bateria-1",
      name: "Batería doméstica",
      kind: "batería",
      status: null,
      note: "Valor de ejemplo",
    };
    expect(deviceStatusLabel(device)).toBe("Sin datos disponibles");
  });
});

describe("hasDevices", () => {
  it("es falso con la lista vacía", () => {
    expect(hasDevices([])).toBe(false);
  });

  it("es verdadero cuando hay al menos un dispositivo", () => {
    const device: DeviceSummary = {
      id: "dev-inversor-1",
      name: "Inversor solar",
      kind: "inversor",
      status: "En línea",
      note: "Valor de ejemplo",
    };
    expect(hasDevices([device])).toBe(true);
  });
});
