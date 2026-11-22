import { describe, expect, it } from "vitest";
import { resolveTheme } from "./theme";

describe("resolveTheme", () => {
  it("respeta el tema guardado en localStorage", () => {
    expect(resolveTheme("dark", false)).toBe("dark");
    expect(resolveTheme("light", true)).toBe("light");
  });

  it("sin tema guardado sigue la preferencia del sistema", () => {
    expect(resolveTheme(null, true)).toBe("dark");
    expect(resolveTheme(null, false)).toBe("light");
  });

  it("ignora valores guardados inválidos y sigue el sistema", () => {
    expect(resolveTheme("rosa", true)).toBe("dark");
    expect(resolveTheme("", false)).toBe("light");
  });
});
