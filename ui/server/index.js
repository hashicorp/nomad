/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

module.exports = function (app, options) {
  const globSync = require('glob').sync;
  const mocks = globSync('./mocks/**/*.js', { cwd: __dirname }).map(require);
  const proxies = globSync('./proxies/**/*.js', { cwd: __dirname }).map(
    require
  );

  // Log proxy requests
  const morgan = require('morgan');
  app.use(morgan('dev'));

  mocks.forEach((route) => route(app, options));
  proxies.forEach((route) => route(app, options));
};
