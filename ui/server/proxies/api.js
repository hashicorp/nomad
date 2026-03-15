/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

const proxyPath = '/v1';

module.exports = function (app, options) {
  // For options, see:
  // https://github.com/nodejitsu/node-http-proxy

  let proxyAddress = options.proxy;

  if (!proxyAddress) {
    return;
  }

  let server = options.httpServer;
  let proxy = require('http-proxy').createProxyServer({
    target: proxyAddress,
    ws: true,
    changeOrigin: true,
  });

  proxy.on('error', function (err, req) {
    // eslint-disable-next-line
    console.error(err, req.url);
  });

  app.use(proxyPath, function (req, res) {
    // include root path in proxied request
    req.url = proxyPath + req.url;
    proxy.web(req, res, { target: proxyAddress });
  });

  server.on('upgrade', function (req) {
    // Set Origin header so Nomad accepts the proxied request.
    // WebSocket proxing is handled by ember-cli.
    // https://github.com/ember-cli/ember-cli/blob/v3.28.5/lib/tasks/server/middleware/proxy-server/index.js#L51
    req.headers.origin = proxyAddress;
  });
};
