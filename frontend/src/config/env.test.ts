import { describe, expect, it } from "vitest";

describe("app config", () => {
  it("exports a usable API base URL", async () => {
    const { appConfig } = await import("./env");
    expect(appConfig.apiBaseUrl).toMatch(/^https?:\/\//);
  });
});
