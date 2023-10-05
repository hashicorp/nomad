/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

const proxyPath = '/v1';
// import PerMessageDeflate from 'ws/lib/permessage-deflate';
// import Receiver from 'ws/lib/receiver';
let {PerMessageDeflate} = require('ui/node_modules/ws/lib/permessage-deflate.js');
// let {Receiver} = require('ui/node_modules/ws/lib/receiver.js');
// let ws = require('ws');
// console.log('====WS', ws);



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
  console.log('proxy2', proxyAddress);
  let proxy = require('http-proxy').createProxyServer({
    target: proxyAddress,
    ws: true,
    changeOrigin: true,
  });

  proxy.on('error', function (err, req) {
    // eslint-disable-next-line
    console.error(err, req.url);
  });

  proxy.on('proxyRes', function (proxyRes, req, res) {
    if (req.upgrade) {  // This checks if it's a WebSocket request
      console.log('Sec-WebSocket-Accept:', proxyRes.headers['sec-websocket-accept']);
    }
  });

  // proxy.on('proxyReqWs', function (proxyReq, req, res) {
  //   console.log('PROXYREQWS', req);
  //   // if (req.upgrade) {  // This checks if it's a WebSocket request
  //   //   console.log('Sec-WebSocket-Accept:', proxyRes.headers['sec-websocket-accept']);
  //   // }
  // });

  // proxy.on('proxyReqWs', (proxyReq, _req, clientSocket, _res, _options) => {

  //   clientSocket.on('error', (error) => {
  //       console.error(`Client socket error:`, error)
  //   })
    
  //   clientSocket.on('close', (hadError) => {
  //       console.log(`Client socket closed${hadError ? ' with error' : ''}.`)
  //   })
    
  //   console.log('PMD,R', PerMessageDeflate, Receiver, typeof PerMessageDeflate, typeof Receiver);

  //   const perMessageDeflate = new PerMessageDeflate({ serverMaxWindowBits: 10, clientMaxWindowBits: 10 }, false)
  //   perMessageDeflate.accept([{}])
  //   const receiver = new Receiver({ isServer: true, extensions: { 'permessage-deflate': perMessageDeflate } })
  //   clientSocket.pipe(receiver)
    
  //   receiver.on('message', (message) => {
  //       console.log(`Client wrote >>>> ${message.toString()}`)
  //       let parsedMsg = JSON.parse(message);
  //       let blockMethodNames = ['method_name_to_block'];
  //       console.log(`Client called >>>> ${parsedMsg.method}`)
  //       if (blockMethodNames.indexOf(parsedMsg.method) > -1) {
  //           clientSocket.end();
  //       }
  //   })
  //   })

  app.use(proxyPath, function (req, res) {
    // include root path in proxied request
    req.url = proxyPath + req.url;
    proxy.web(req, res, { target: proxyAddress });
  });

  server.on('upgrade', function (req, socket, head) {
    console.log('REQHEADERS', req.url, req.headers);

    if (
      req.url.startsWith('/v1/client/allocation') &&
      req.url.includes('exec?')
    ) {
      // req.headers.origin = proxyAddress;
      // req.headers.host = "localhost:4646";
      // console.log("yeah!", proxyAddress);
      proxy.ws(req, socket, head, { target: proxyAddress });
    }
  });
};
