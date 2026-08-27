// @ts-check

const {defineConfig, devices} = require('@playwright/test');

module.exports = defineConfig({
    testDir: './tests/browser',
    outputDir: './output/playwright/results',
    reporter: [['list'], ['html', {outputFolder: './output/playwright/report', open: 'never'}]],
    use: {
        baseURL: 'http://127.0.0.1:18080',
        screenshot: 'only-on-failure',
        trace: 'retain-on-failure',
    },
    webServer: {
        command: 'go run ./internal/browserfixture',
        url: 'http://127.0.0.1:18080/browser-login/',
        reuseExistingServer: false,
        timeout: 120000,
    },
    projects: [
        {name: 'chromium', use: {...devices['Desktop Chrome']}},
    ],
});
