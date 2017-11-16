import Ember from 'ember';
import { task } from 'ember-concurrency';
import { logger } from 'nomad-ui/utils/classes/log';
import WindowResizable from 'nomad-ui/mixins/window-resizable';

const { Component, computed, inject, run } = Ember;

export default Component.extend(WindowResizable, {
  token: inject.service(),

  classNames: ['boxed-section', 'task-log'],

  allocation: null,
  task: null,

  didReceiveAttrs() {
    if (this.get('allocation') && this.get('task')) {
      this.send('toggleStream');
    }
  },

  didInsertElement() {
    this.fillAvailableHeight();
  },

  windowResizeHandler() {
    run.once(this, this.fillAvailableHeight);
  },

  fillAvailableHeight() {
    // This math is arbitrary and far from bulletproof, but the UX
    // of having the log window fill available height is worth the hack.
    const cliWindow = this.$('.cli-window');
    cliWindow.height(window.innerHeight - cliWindow.offset().top - 25);
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
      const cliWindow = this.$('.cli-window');
      cliWindow.scrollTop(cliWindow[0].scrollHeight);
    });
  }),

  stream: task(function*() {
    this.get('logger').on('tick', () => {
      run.scheduleOnce('afterRender', () => {
        const cliWindow = this.$('.cli-window');
        cliWindow.scrollTop(cliWindow[0].scrollHeight);
      });
    });

    yield this.get('logger').startStreaming();
    this.get('logger').off('tick');
  }),

  willDestroy() {
    this.get('logger').stop();
  },

  actions: {
    setMode(mode) {
      this.get('logger').stop();
      this.set('mode', mode);
      this.get('stream').perform();
    },
    toggleStream() {
      if (this.get('logger.isStreaming')) {
        this.get('logger').stop();
      } else {
        this.get('stream').perform();
      }
    },
  },
});
