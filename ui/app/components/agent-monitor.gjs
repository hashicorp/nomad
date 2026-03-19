/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { service } from '@ember/service';
import { assert } from '@ember/debug';
import { on } from '@ember/modifier';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import PowerSelect from 'ember-power-select/components/power-select';
import StreamingFile from 'nomad-ui/components/streaming-file';
import Log from 'nomad-ui/utils/classes/log';

const LEVELS = ['error', 'warn', 'info', 'debug', 'trace'];

export default class AgentMonitor extends Component {
  @service token;

  levels = LEVELS;
  monitorUrl = '/v1/agent/monitor';

  @tracked level = this.args.level ?? LEVELS[2];
  @tracked isStreaming = this.args.isStreaming ?? true;
  @tracked logger = null;

  get monitorParams() {
    assert(
      'Provide a client OR a server to AgentMonitor, not both.',
      this.args.server != null || this.args.client != null,
    );

    const type = this.args.server ? 'server_id' : 'client_id';
    const id = this.args.server ? this.args.server.id : this.args.client?.id;

    const params = {
      log_level: this.level,
      [type]: id,
    };

    if (this.args.server) {
      params.region = this.args.server.region;
    }

    return params;
  }

  capitalizeLevel = (value) => {
    if (!value) return '';
    return `${value.charAt(0).toUpperCase()}${value.slice(1)}`;
  };

  initialize = () => {
    this.updateLogger();
  };

  updateLogger = () => {
    let currentTail = this.logger ? this.logger.tail : '';
    if (currentTail) {
      currentTail += `\n...changing log level to ${this.level}...\n\n`;
    }

    this.logger = Log.create({
      logFetch: (url) => this.token.authorizedRequest(url),
      params: this.monitorParams,
      url: this.monitorUrl,
      tail: currentTail,
    });
  };

  setLevel = (level) => {
    this.logger?.stop();
    this.level = level;
    this.args.onLevelChange?.(level);
    this.updateLogger();
  };

  toggleStream = () => {
    this.isStreaming = !this.isStreaming;
  };

  <template>
    <div class="boxed-section" {{didInsert this.initialize}}>
      <div class="boxed-section-head" data-test-level-switcher-parent>
        <PowerSelect
          data-test-level-switcher
          @ariaLabel="label-level-switcher"
          @ariaLabelledBy="label-level-switcher"
          @tagName="div"
          @triggerClass="is-compact pull-left"
          @options={{this.levels}}
          @selected={{this.level}}
          @searchEnabled={{false}}
          @onChange={{this.setLevel}}
          as |level|
        >
          <span class="ember-power-select-prefix">Level:
          </span>{{this.capitalizeLevel level}}
        </PowerSelect>
        <button
          data-test-toggle
          class="button is-white is-compact pull-right"
          {{on "click" this.toggleStream}}
          type="button"
          title={{if this.logger.isStreaming "Stop" "Start"}}
        >
          <HdsIcon
            @name={{if this.logger.isStreaming "pause" "play"}}
            @isInline={{true}}
          />
        </button>
      </div>
      <div data-test-log-box class="boxed-section-body is-dark is-full-bleed">
        <StreamingFile
          @logger={{this.logger}}
          @isStreaming={{this.isStreaming}}
        />
      </div>
    </div>
  </template>
}
