/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-env node */
/* eslint-disable no-console */

const fs = require('fs');
const path = require('path');

class JsonReporter {
  constructor(out, socket, config) {
    this.out = out || process.stdout;
    this.results = [];

    // Get output file from Testem config, which is set by the --json-report=path argument
    this.outputFile = config?.fileOptions?.custom_report_file;
    this.generateReport = !!this.outputFile;

    if (this.generateReport) {
      console.log(
        `[Reporter] Initializing with output file: ${this.outputFile}`
      );

      try {
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
    } else {
      console.log('[Reporter] No report file configured, skipping JSON output');
    }

    process.on('SIGINT', () => {
      console.log('[Reporter] Received SIGINT, finishing up...');
      this.finish();
      process.exit(0);
    });

    this.testCounter = 0;
    this.startTime = Date.now();
  }

  filterLogs(logs) {
    return logs.filter((log) => {
      // Filter out token-related logs
      if (
        log.text &&
        (log.text.includes('Accessor:') ||
          log.text.includes('log in with a JWT') ||
          log.text === 'TOKENS:' ||
          log.text === '=====================================')
      ) {
        return false;
      }

      // Keep non-warning logs that aren't token-related
      return log.type !== 'warn';
    });
  }

  report(prefix, data) {
    if (!data || !data.name) {
      console.log(`[Reporter] Skipping invalid test result: ${data.name}`);
      return;
    }

    this.testCounter++;
    console.log(`[Reporter] Test #${this.testCounter}: ${data.name}`);

    const partitionMatch = data.name.match(/^Exam Partition (\d+) - (.*)/);

    const result = {
      name: partitionMatch ? partitionMatch[2] : data.name.trim(),
      partition: partitionMatch ? parseInt(partitionMatch[1], 10) : null,
      browser: prefix,
      passed: !data.failed,
      duration: data.runDuration,
      error: data.failed ? data.error : null,
      logs: this.filterLogs(data.logs || []),
    };

    if (result.passed) {
      console.log('- [PASS]');
    } else {
      console.log('- [FAIL]');
      console.log('- Error:', result.error);
      console.log('- Logs:', result.logs);
    }

    this.results.push(result);
  }

  writeCurrentResults() {
    console.log('[Reporter] Writing current results...');
    try {
      const passed = this.results.filter((r) => r.passed).length;
      const failed = this.results.filter((r) => !r.passed).length;
      const total = this.results.length;
      const duration = Date.now() - this.startTime;

      const output = {
        summary: { total, passed, failed },
        timestamp: new Date().toISOString(),
        duration,
        tests: this.results,
      };

      if (this.generateReport) {
        fs.writeFileSync(this.outputFile, JSON.stringify(output, null, 2));
      }

      // Print a summary
      console.log('\n[Reporter] Test Summary:');
      console.log(`- Total:  ${total}`);
      console.log(`- Passed: ${passed}`);
      console.log(`- Failed: ${failed}`);
      console.log(`- Duration: ${duration}ms`);
      if (failed > 0) {
        console.log('\n[Reporter] Failed Tests:');
        this.results
          .filter((r) => !r.passed)
          .forEach((r) => {
            console.log(`❌ ${r.name}`);
            if (r.error) {
              console.error(r.error);
            }
          });
      }

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
