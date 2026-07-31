/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import WatcherSocketAdapter from 'nomad-ui/utils/classes/watcher-socket-adapter';
import { getOwner } from '@ember/owner';
import { service } from '@ember/service';

const WATCHER_PROTOCOL_NAME = 'nomad-watcher';

export default class WatcherFetchService extends Service {
  @service system;

  // fetch fetches the given request over a websocket and returns
  // a promise response.
  fetch(hash) {
    // Only GET requests are supported.
    if (hash.method !== 'GET') {
      return new Promise(() => {
        throw new DOMException(
          'unsupported request method: ' + hash.method,
          'UnsupportedMethodError',
        );
      });
    }
    // Collect all the information needed for the URL.
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const applicationAdapter = getOwner(this).lookup('adapter:application');
    const host = applicationAdapter.host || window.location.host;
    const prefix = `${protocol}//${host}`;
    const region = this.system.activeRegion;
    const token =
      typeof hash.headers === 'object' ? hash.headers['X-Nomad-Token'] : null;

    // Build the full URL for the websocket.
    const fullUrl =
      hash.url.charAt(0) === '/' ? `${prefix}${hash.url}` : hash.url;

    // Parse the URL, enable the handshake if a token was provided,
    // and set the region if one is available.
    let url = new URL(fullUrl);
    if (token) {
      url.searchParams.append('ws_handshake', 'true');
    }
    if (region) {
      url.searchParams.append('region', region);
    }

    // Create the websocket using the built url.
    const socket = new WebSocket(url.toString(), WATCHER_PROTOCOL_NAME);

    // Finally, create a new watcher socket adapter for the
    // websocket and return the response promise.
    return new WatcherSocketAdapter(socket, token, hash.signal).response();
  }
}
