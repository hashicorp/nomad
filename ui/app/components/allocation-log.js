import Ember from 'ember';
import { logger } from 'nomad-ui/utils/classes/log';
import { task } from 'ember-concurrency';

const { Component, computed, inject, run } = Ember;

export default Component.extend({
  token: inject.service(),

  classNames: ['boxed-section'],

  allocation: null,
  task: null,

  didReceiveAttrs() {
    if (this.get('allocation') && this.get('task')) {
      this.send('toggleStream');
    }
  },

  mode: 'stdout',

  logUrl: computed('allocation.id', 'allocation.node.httpAddr', function() {
    const address = this.get('allocation.node.httpAddr');
    const allocation = this.get('allocation.id');

    return `//${address}/v1/client/fs/logs/${allocation}`;
  }),

  logParams: computed('task', 'mode', function() {
    return {
      task: this.get('task'),
      type: this.get('mode'),
    };
  }),

  logger: logger('logUrl', 'logParams', function() {
    const token = this.get('token');
    return token.authorizedRequest.bind(token);
  }),

  head: task(function*() {
    yield this.get('logger.gotoHead').perform();
    run.scheduleOnce('afterRender', () => {
      this.$('.cli-window').scrollTop(0);
    });
  }),

  tail: task(function*() {
    yield this.get('logger.gotoTail').perform();
    run.scheduleOnce('afterRender', () => {
      this.$('.cli-window').scrollTop(this.$('.cli-window')[0].scrollHeight);
    });
  }),

  stream: task(function*() {
    this.get('logger').on('tick', () => {
      var cliWindow = this.$('.cli-window');
      run.scheduleOnce('afterRender', () => {
        cliWindow.scrollTop(cliWindow[0].scrollHeight);
      });
    });

    yield this.get('logger').startStreaming();
    this.get('logger').off('tick');
  }),

  actions: {
    setMode(mode) {
      this.send('stopStreaming');
      this.set('mode', mode);
    },
    stopStreaming() {
      this.get('logger').stop();
    },
    toggleStream() {
      if (this.get('logger.isStreaming')) {
        this.send('stopStreaming');
      } else {
        this.get('stream').perform();
      }
    },
  },
});
