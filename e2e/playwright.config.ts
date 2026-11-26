import { defineConfig } from '@playwright/test';

// PW_REPORT_DIR lets scripts/test-all.sh and scripts/capture-proof.sh save reports under proof/.
const reportDir = process.env.PW_REPORT_DIR ?? 'test-results/report';

export default defineConfig({
  testDir: './tests',
  timeout: 120_000,
  expect: { timeout: 30_000 },
  workers: 1,
  reporter: [
    ['list'],
    ['html', { outputFolder: `${reportDir}/html`, open: 'never' }],
    ['junit', { outputFile: `${reportDir}/playwright-junit.xml` }]
  ],
  use: { trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  outputDir: `${reportDir}/artifacts`
});
