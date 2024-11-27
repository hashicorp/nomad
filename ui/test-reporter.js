/* eslint-env node */
/* eslint-disable no-console */

const fs = require('fs');
const path = require('path');

class JsonReporter {
  constructor(out, socket, config) {
    // Prevent double initialization
    if (JsonReporter.instance) {
      return JsonReporter.instance;
    }
    JsonReporter.instance = this;

    this.out = out || process.stdout;
    this.results = [];

    // Get output file from Testem config
    this.outputFile = config.fileOptions.report_file || 'test-results.json';

    console.log(`[Reporter] Initializing with output file: ${this.outputFile}`);

    try {
      // Ensure output directory exists
      fs.mkdirSync(path.dirname(this.outputFile), { recursive: true });

      // Initialize the results file
      fs.writeFileSync(
        this.outputFile,
        JSON.stringify(
          {
            summary: { total: 0, passed: 0, failed: 0 },
            timestamp: new Date().toISOString(),
            tests: [],
          },
          null,
          2
        )
      );
      console.log('[Reporter] Initialized results file');
    } catch (err) {
      console.error('[Reporter] Error initializing results file:', err);
    }

    process.on('SIGINT', () => {
      console.log('[Reporter] Received SIGINT, finishing up...');
      this.finish();
      process.exit(0);
    });
  }

  report(prefix, data) {
    if (!data || !data.name) {
      console.log(`[Reporter] Skipping invalid test result: ${data.name}`);
      return;
    }

    console.log(`[Reporter] Processing test: ${data.name}`);

    const result = {
      name: data.name.trim(),
      browser: prefix,
      passed: !data.failed,
      duration: data.runDuration,
      error: data.failed ? data.error : null,
      logs: (data.logs || []).filter((log) => log.type !== 'warn'),
    };

    this.results.push(result);
  }

  writeCurrentResults() {
    console.log('[Reporter] Writing current results...');
    try {
      const output = {
        summary: {
          total: this.results.length,
          passed: this.results.filter((r) => r.passed).length,
          failed: this.results.filter((r) => !r.passed).length,
        },
        timestamp: new Date().toISOString(),
        tests: this.results,
      };

      fs.writeFileSync(this.outputFile, JSON.stringify(output, null, 2));
      console.log('[Reporter] Successfully wrote results');
    } catch (err) {
      console.error('[Reporter] Error writing results:', err);
    }
  }

  finish() {
    console.log('[Reporter] Finishing up...');
    this.writeCurrentResults();
    console.log('[Reporter] Done.');
  }
}

module.exports = JsonReporter;
