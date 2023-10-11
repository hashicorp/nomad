/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Service from '@ember/service';
import config from 'nomad-ui/config/environment';
import { getOwner } from '@ember/application';
import { inject as service } from '@ember/service';

export default class SocketsService extends Service {
  @service system;

  getTaskStateSocket(taskState, command) {
    const mirageEnabled =
      config.environment !== 'production' &&
      config['ember-cli-mirage'] &&
      config['ember-cli-mirage'].enabled !== false;

    if (mirageEnabled) {
      return new Object({
        messageDisplayed: false,

        send(e) {
          if (!this.messageDisplayed) {
            this.messageDisplayed = true;
            this.onmessage({
              data: `{"stdout":{"data":"${btoa(
                'unsupported in Mirage\n\r'
              )}"}}`,
            });
          } else {
            this.onmessage({ data: e.replace('stdin', 'stdout') });
          }
        },
      });
    } else {
      const shouldForward = config.APP.deproxyWebsockets;

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';

      // FIXME: Temporary, local ember implementation to get around websocket proxy issues duiring development.
      let prefix;
      const region = this.system.activeRegion;
      if (!shouldForward) {
        const applicationAdapter = getOwner(this).lookup('adapter:application');
        prefix = `${
          applicationAdapter.host || window.location.host
        }/${applicationAdapter.urlPrefix()}`;
      } else {
        prefix = 'localhost:4646/v1';
      }

      return new WebSocket(
        `${protocol}//${prefix}/client/allocation/${taskState.allocation.id}` +
          `/exec?task=${taskState.name}&tty=true&ws_handshake=true` +
          (region ? `&region=${region}` : '') +
          `&command=${encodeURIComponent(`["${command}"]`)}`
      );
    }
  }
}
