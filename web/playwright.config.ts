import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  workers: 1,
  webServer: [
    {
      command: "../scripts/browser-server.sh",
      url: "http://127.0.0.1:18080/api/health",
      timeout: 120000,
      reuseExistingServer: false,
    },
    {
      command:
        "YANETVIZ_TEST_PORT=18081 YANETVIZ_TEST_PASSWORD=test-admin-password YANETVIZ_TEST_PUBLIC_VIEW=true ../scripts/browser-server.sh",
      url: "http://127.0.0.1:18081/api/health",
      timeout: 120000,
      reuseExistingServer: false,
    },
  ],
  use: {
    baseURL: "http://127.0.0.1:18080",
    viewport: { width: 1440, height: 1000 },
  },
});
