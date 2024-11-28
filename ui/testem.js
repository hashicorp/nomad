/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';
const JsonReporter = require('./test-reporter');

const config = {
  test_page: 'tests/index.html?hidepassed',
  disable_watching: true,
  launch_in_ci: ['Chrome'],
  launch_in_dev: ['Chrome'],
  browser_start_timeout: 120,
  parallel: -1,
  framework: 'qunit',
  reporter: JsonReporter,
  custom_report_file: 'test-results/test-results.json',
  // report_file: 'test-results/test-results.json',
  // NOTE: See https://github.com/testem/testem/issues/1073, report_file + custom reporter results in double output.
  debug: true,

  browser_args: {
    // New format in testem/master, but not in a release yet
    // Chrome: {
    //   ci: ['--headless', '--disable-gpu', '--remote-debugging-port=9222', '--window-size=1440,900'],
    // },
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

module.exports = config;
