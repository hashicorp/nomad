/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { action, computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import RSVP from 'rsvp';
import { logger } from 'nomad-ui/utils/classes/log';
import timeout from 'nomad-ui/utils/timeout';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

class MockAbortController {
  abort() {
    /* noop */
  }
}

@classic
@classNames('boxed-section', 'task-log')
export default class TaskLog extends Component {
  @service token;
  @service userSettings;

  allocation = null;
  task = null;

  // When true, request logs from the server agent
  useServer = false;

  // When true, logs cannot be fetched from either the client or the server
  noConnection = false;

  clientTimeout = 1000;
  serverTimeout = 5000;

  isStreaming = true;
  streamMode = 'streaming';

  shouldFillHeight = true;

  @localStorageProperty('nomadShouldWrapCode', false) wrapped;

  @alias('userSettings.logMode') mode;

  @computed('allocation.{id,node.httpAddr}', 'useServer')
  get logUrl() {
    const address = this.get('allocation.node.httpAddr');
    const allocation = this.get('allocation.id');

    const url = `/v1/client/fs/logs/${allocation}`;
    return this.useServer ? url : `//${address}${url}`;
  }

  @computed('task', 'mode')
  get logParams() {
    return {
      task: this.task,
      type: this.mode,
    };
  }

  @logger('logUrl', 'logParams', function logFetch() {
    // If the log request can't settle in one second, the client
    // must be unavailable and the server should be used instead

    const aborter = window.AbortController
      ? new AbortController()
      : new MockAbortController();
    const timing = this.useServer ? this.serverTimeout : this.clientTimeout;

    // Capture the state of useServer at logger create time to avoid a race
    // between the stdout logger and stderr logger running at once.
    const useServer = this.useServer;
    return (url) =>
      RSVP.race([
        this.token.authorizedRequest(url, { signal: aborter.signal }),
        timeout(timing),
      ]).then(
        (response) => {
          return response;
        },
        (error) => {
          aborter.abort();
          if (useServer) {
            this.set('noConnection', true);
          } else {
            this.send('failoverToServer');
          }
          throw error;
        }
      );
  })
  logger;

  @action
  setMode(mode) {
    if (this.mode === mode) return;
    this.logger.stop();
    this.set('mode', mode);
  }

  @action
  toggleStream() {
    this.set('streamMode', 'streaming');
    this.toggleProperty('isStreaming');
  }

  @action
  gotoHead() {
    this.set('streamMode', 'head');
    this.set('isStreaming', false);
  }

  @action
  gotoTail() {
    this.set('streamMode', 'tail');
    this.set('isStreaming', false);
  }

  @action
  failoverToServer() {
    this.set('useServer', true);
  }

  @action toggleWrap() {
    this.toggleProperty('wrapped');
    return false;
  }
}
