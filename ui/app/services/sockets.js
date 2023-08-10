/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const applicationAdapter = getOwner(this).lookup('adapter:application');
      const prefix = `${
        applicationAdapter.host || window.location.host
      }/${applicationAdapter.urlPrefix()}`;
      const region = this.system.activeRegion;

      return new WebSocket(
        `${protocol}//${prefix}/client/allocation/${taskState.allocation.id}` +
          `/exec?task=${taskState.name}&tty=true&ws_handshake=true` +
          (region ? `&region=${region}` : '') +
          `&command=${encodeURIComponent(`["${command}"]`)}`
      );
    }
  }
}
