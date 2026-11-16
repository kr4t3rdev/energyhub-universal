import type { SiteOverview } from "./types";

/**
 * Frontera entre la pantalla y el origen de datos.
 * En el futuro, una implementación contra la API del backend (no `/health`,
 * que solo indica vida del proceso) sustituirá a la fixture sin cambiar la página.
 */
export interface SiteOverviewProvider {
  getSiteOverview(): Promise<SiteOverview>;
}

export class FixtureSiteOverviewProvider implements SiteOverviewProvider {
  async getSiteOverview(): Promise<SiteOverview> {
    const { exampleSite } = await import("./example");
    return exampleSite;
  }
}

export const siteOverviewProvider: SiteOverviewProvider = new FixtureSiteOverviewProvider();
