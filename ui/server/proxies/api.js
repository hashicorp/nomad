/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

const proxyPath = '/v1';

module.exports = function (app, options) {
  // For options, see:
  // https://github.com/nodejitsu/node-http-proxy

  // This is probably not safe to do, but it works for now.
  let cacheKey = `${options.project.configPath()}|${options.environment}`;
  let config = options.project.configCache.get(cacheKey);

  // Disable the proxy completely when Mirage is enabled. No requests to the API
  // will be being made, and having the proxy attempt to connect to Nomad when it
  // is not running can result in socket max connections that block the livereload
  // server from reloading.
  if (config['ember-cli-mirage'].enabled !== false) {
    options.ui.writeInfoLine('Mirage is enabled. Not starting proxy');
    delete options.proxy;
    return;
  }

  let proxyAddress = options.proxy;

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

  server.on('upgrade', function (req, socket, head) {
    if (
      req.url.startsWith('/v1/client/allocation') &&
      req.url.includes('exec?')
    ) {
      req.headers.origin = proxyAddress;
      proxy.ws(req, socket, head, { target: proxyAddress });
    }
  });
};
