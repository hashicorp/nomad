'use strict';

// Issue to improve: https://github.com/hashicorp/nomad/issues/7465
const proxyPath = '/v1';

module.exports = function(app, options) {
  // For options, see:
  // https://github.com/nodejitsu/node-http-proxy

  let proxyAddress = options.proxy;

  let server = options.httpServer;
  let proxy = require('http-proxy').createProxyServer({
    target: proxyAddress,
    ws: true,
    changeOrigin: true,
  });

  proxy.on('error', function(err, req) {
    // eslint-disable-next-line
    console.error(err, req.url);
  });

  app.use(proxyPath, function(req, res) {
    // include root path in proxied request
    req.url = proxyPath + req.url;
    proxy.web(req, res, { target: proxyAddress });
  });

  server.on('upgrade', function(req, socket, head) {
    if (req.url.startsWith('/v1/client/allocation') && req.url.includes('exec?')) {
      req.headers.origin = proxyAddress;
      proxy.ws(req, socket, head, { target: proxyAddress });
    }
  });
};
