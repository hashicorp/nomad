/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { fn, array } from '@ember/helper';
import { tracked } from '@glimmer/tracking';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { eq } from 'ember-truth-helpers';
import {
  HdsFormToggleField,
  HdsIcon,
} from '@hashicorp/design-system-components/components';
import RSVP from 'rsvp';
import { logger } from 'nomad-ui/utils/classes/log';
import timeout from 'nomad-ui/utils/timeout';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import StreamingFile from 'nomad-ui/components/streaming-file';

export default class TaskLog extends Component {
  @service token;
  @service userSettings;
  @service abilities;

  @tracked useServer = false;
  @tracked noConnection = false;
  @tracked logsDisabled = false;

  clientTimeout = 1000;
  serverTimeout = 5000;

  @tracked isStreaming = true;
  @tracked streamMode = 'streaming';

  shouldFillHeight = true;

  @localStorageProperty('nomadShouldWrapCode', false) wrapped;

  get mode() {
    return this.userSettings.logMode;
  }

  set mode(value) {
    this.userSettings.logMode = value;
  }

  get logUrl() {
    let address;
    const allocation = this.args.allocation?.id;
    if (this.abilities.can('read client')) {
      address = this.args.allocation?.node?.httpAddr;
    }
    const url = `/v1/client/fs/logs/${allocation}`;
    return this.useServer ? url : address ? `//${address}${url}` : url;
  }

  get logParams() {
    return {
      task: this.args.task,
      type: this.mode,
    };
  }

  @logger('logUrl', 'logParams', function logFetch() {
    const aborter = new AbortController();
    const timing = this.useServer ? this.serverTimeout : this.clientTimeout;
    const useServer = this.useServer;

    return (url) =>
      RSVP.race([
        this.token.authorizedRequest(url, { signal: aborter.signal }),
        timeout(timing),
      ]).then(
        (response) => {
          if (response.status === 404) {
            this.logsDisabled = true;
          }
          return response;
        },
        (error) => {
          aborter.abort();
          if (useServer) {
            this.noConnection = true;
          } else {
            this.failoverToServer();
          }
          throw error;
        },
      );
  })
  logger;

  setMode = (mode) => {
    if (this.mode === mode) return;
    this.logger.stop();
    this.mode = mode;
  };

  toggleStream = () => {
    this.streamMode = 'streaming';
    this.isStreaming = !this.isStreaming;
  };

  gotoHead = () => {
    this.streamMode = 'head';
    this.isStreaming = false;
  };

  gotoTail = () => {
    this.streamMode = 'tail';
    this.isStreaming = false;
  };

  failoverToServer = () => {
    this.useServer = true;
  };

  toggleWrap = () => {
    this.wrapped = !this.wrapped;
    return false;
  };

  dismissNoConnection = () => {
    this.noConnection = false;
  };

  dismissLogsDisabled = () => {
    this.logsDisabled = false;
  };

  <template>
    <div class="boxed-section task-log" ...attributes>
      {{#if this.noConnection}}
        <div data-test-connection-error class="notification is-error">
          <div class="columns">
            <div class="column">
              <h3 class="title is-4">Cannot fetch logs</h3>
              <p>The logs for this task are inaccessible. Check the condition of
                the node the allocation is on.</p>
            </div>
            <div class="column is-centered is-minimum">
              <button
                data-test-connection-error-dismiss
                class="button is-danger"
                {{on "click" this.dismissNoConnection}}
                type="button"
              >Okay</button>
            </div>
          </div>
        </div>
      {{/if}}
      {{#if this.logsDisabled}}
        <div data-test-connection-error class="notification is-error">
          <div class="columns">
            <div class="column">
              <h3 class="title is-4">Cannot fetch logs</h3>
              <p>Logs unavailable. Log collection may be disabled.</p>
            </div>
            <div class="column is-centered is-minimum">
              <button
                data-test-connection-error-dismiss
                class="button is-danger"
                {{on "click" this.dismissLogsDisabled}}
                type="button"
              >Okay</button>
            </div>
          </div>
        </div>
      {{/if}}
      <div class="boxed-section-head task-log-head">
        <span>
          <button
            data-test-log-action="stdout"
            class="button {{if (eq this.mode 'stdout') 'is-info'}}"
            {{on "click" (fn this.setMode "stdout")}}
            type="button"
          >stdout</button>
          <button
            data-test-log-action="stderr"
            class="button {{if (eq this.mode 'stderr') 'is-danger'}}"
            {{on "click" (fn this.setMode "stderr")}}
            type="button"
          >stderr</button>
        </span>
        <span class="pull-right">
          <span class="header-toggle">
            <HdsFormToggleField
              {{keyboardShortcut
                label="Toggle word wrap"
                action=this.toggleWrap
                pattern=(array "w" "w")
                menuLevel=true
              }}
              checked={{this.wrapped}}
              {{on "change" this.toggleWrap}}
              data-test-word-wrap-toggle
              as |F|
            >
              <F.Label>Word Wrap</F.Label>
            </HdsFormToggleField>
          </span>
          <button
            data-test-log-action="head"
            class="button is-white"
            {{on "click" this.gotoHead}}
            type="button"
          >Head</button>
          <button
            data-test-log-action="tail"
            class="button is-white"
            {{on "click" this.gotoTail}}
            type="button"
          >Tail</button>
          <button
            data-test-log-action="toggle-stream"
            class="button is-white"
            {{on "click" this.toggleStream}}
            type="button"
            title="{{if this.logger.isStreaming 'Stop' 'Start'}} log streaming"
          >
            <HdsIcon @name={{if this.logger.isStreaming "pause" "play"}} />
          </button>
        </span>
      </div>
      <div data-test-log-box class="boxed-section-body is-dark is-full-bleed">
        <StreamingFile
          @logger={{this.logger}}
          @mode={{this.streamMode}}
          @isStreaming={{this.isStreaming}}
          @shouldFillHeight={{this.shouldFillHeight}}
          @wrapped={{this.wrapped}}
        />
      </div>
    </div>
  </template>
}
