/* eslint-env node */
module.exports = {
  test_page: 'tests/index.html?hidepassed',
  disable_watching: true,
  launch_in_ci: ['Chrome'],
  launch_in_dev: ['Chrome'],
  browser_args: {
    // New format in testem/master, but not in a release yet
    // Chrome: {
    //   ci: ['--headless', '--disable-gpu', '--remote-debugging-port=9222', '--window-size=1440,900'],
    // },
    Chrome: {
      mode: 'ci',
      args: [
        '--headless',
        '--no-sandbox',
        '--disable-gpu',
        '--remote-debugging-port=9222',
        '--window-size=1440,900',
      ],
    },
  },
};
