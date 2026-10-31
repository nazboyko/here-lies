import { defineConfig } from "@playwright/test";

// Every test runs against the real server binary and the real local model.
// Three servers: the normal one (on all interfaces, so the LAN test can
// reach it), one with a model that is not pulled, and one pointed at a
// closed Ollama port.
const bin = "../bin/herelies";

export default defineConfig({
  projects: [
    { name: "e2e", testDir: "tests" },
    { name: "shots", testDir: "shots" }, // images for the docs: make shots
  ],
  outputDir: "out/test-results",
  timeout: 120_000,
  expect: { timeout: 60_000 },
  workers: 1, // the model writes one obituary at a time
  reporter: [["list"]],
  use: { baseURL: "http://localhost:8788", acceptDownloads: true },
  webServer: [
    {
      command: bin,
      env: { HERELIES_ADDR: "0.0.0.0:8788" },
      url: "http://127.0.0.1:8788/api/health",
      reuseExistingServer: false,
    },
    {
      command: bin,
      env: { HERELIES_ADDR: "127.0.0.1:8789", HERELIES_MODEL: "missing-model" },
      url: "http://127.0.0.1:8789/api/health",
      reuseExistingServer: false,
    },
    {
      command: bin,
      env: { HERELIES_ADDR: "127.0.0.1:8790", HERELIES_OLLAMA: "http://127.0.0.1:9" },
      url: "http://127.0.0.1:8790/api/health",
      reuseExistingServer: false,
    },
  ],
});
