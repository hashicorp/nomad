/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { computed } from '@ember/object';
import { assert } from '@ember/debug';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import Log from 'nomad-ui/utils/classes/log';

const LEVELS = ['error', 'warn', 'info', 'debug', 'trace'];

@classic
@tagName('')
export default class AgentMonitor extends Component {
  @service token;

  client = null;
  server = null;
  level = LEVELS[2];
  onLevelChange() {}

  levels = LEVELS;
  monitorUrl = '/v1/agent/monitor';
  isStreaming = true;
  logger = null;

  @computed('client.id', 'level', 'server.{id,region}')
  get monitorParams() {
    assert(
      'Provide a client OR a server to AgentMonitor, not both.',
      this.server != null || this.client != null
    );

    const type = this.server ? 'server_id' : 'client_id';
    const id = this.server ? this.server.id : this.client && this.client.id;

    const params = {
      log_level: this.level,
      [type]: id,
    };

    if (this.server) {
      params.region = this.server.region;
    }

    return params;
  }

  didInsertElement() {
    super.didInsertElement(...arguments);
    this.updateLogger();
  }

  updateLogger() {
    let currentTail = this.logger ? this.logger.tail : '';
    if (currentTail) {
      currentTail += `\n...changing log level to ${this.level}...\n\n`;
    }
    this.set(
      'logger',
      Log.create({
        logFetch: (url) => this.token.authorizedRequest(url),
        params: this.monitorParams,
        url: this.monitorUrl,
        tail: currentTail,
      })
    );
  }

  setLevel(level) {
    this.logger.stop();
    this.set('level', level);
    this.onLevelChange(level);
    this.updateLogger();
  }

  toggleStream() {
    this.set('streamMode', 'streaming');
    this.toggleProperty('isStreaming');
  }
}
