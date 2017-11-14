import Ember from 'ember';
import { logger } from 'nomad-ui/utils/classes/log';
import { task } from 'ember-concurrency';

const { Controller, computed, inject, run, $ } = Ember;

export default Controller.extend({
  token: inject.service(),

  mode: 'stdout',

  logger: logger('logUrl', 'logParams', function() {
    const token = this.get('token');
    return token.authorizedRequest.bind(token);
  }),

  allocation: computed.alias('model.allocation'),
  logUrl: computed('allocation.id', 'allocation.node.httpAddr', 'model.name', 'mode', function() {
    const address = this.get('allocation.node.httpAddr');
    const allocation = this.get('allocation.id');

    // return `//${address}/v1/client/fs/logs/${allocation}`;
    return `//127.0.0.1:4200/v1/client/fs/logs/${allocation}`;
  }),

  logParams: computed('model.name', 'mode', function() {
    return {
      task: this.get('model.name'),
      type: this.get('mode'),
    };
  }),

  head: task(function*() {
    yield this.get('stdout.gotoHead').perform();
    run.scheduleOnce('afterRender', () => {
      $('#logs').scrollTop(0);
    });
  }),

  tail: task(function*() {
    yield this.get('stdout.gotoTail').perform();
    run.scheduleOnce('afterRender', () => {
      $('#logs').scrollTop($('#logs')[0].scrollHeight);
    });
  }),

  stream: task(function*() {
    this.get('stdout').on('tick', () => {
      run.scheduleOnce('afterRender', () => {
        $('#logs').scrollTop($('#logs')[0].scrollHeight);
      });
    });

    yield this.get('stdout').startStreaming();
    this.get('stdout').off('tick');
  }),

  actions: {
    setMode(mode) {
      this.send('stopStreaming');
      this.set('mode', mode);
    },
    stream() {
      this.streamLog();
    },
    stopStreaming() {
      this.get('logger').stop();
    },
  },
});
