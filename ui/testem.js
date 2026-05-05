/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

const JsonReporter = require('./test-reporter');

/**
 * Get the path for the test results file based on the command line arguments
 * @returns {string} The path to the test results file
 */
const getReportPath = () => {
  if (process.env.JSON_REPORT_PATH) {
    return process.env.JSON_REPORT_PATH;
  }

  const jsonReportArg = process.argv.find((arg) =>
    arg.startsWith('--json-report='),
  );
  if (jsonReportArg) {
    return jsonReportArg.split('=')[1];
  }
  return null;
};

module.exports = {
  test_page: 'tests/index.html?hidepassed',
  disable_watching: true,
  launch_in_ci: ['Chrome'],
  launch_in_dev: ['Chrome'],
  browser_start_timeout: 120,
  parallel: -1,
  framework: 'qunit',
  reporter: JsonReporter,
  custom_report_file: getReportPath(),
  // NOTE: we output this property as custom_report_file instead of report_file.
  // See https://github.com/testem/testem/issues/1073, report_file + custom reporter results in double output.
  debug: true,

  browser_args: {
    Chrome: {
      ci: [
        // --no-sandbox is needed when running Chrome inside a container
        process.env.CI ? '--no-sandbox' : null,
        '--headless',
        '--disable-dev-shm-usage',
        '--disable-software-rasterizer',
        '--mute-audio',
        '--remote-debugging-port=0',
        '--window-size=1440,900',
      ].filter(Boolean),
    },
  },
};
