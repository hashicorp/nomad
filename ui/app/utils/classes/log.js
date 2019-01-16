import Ember from 'ember';
import { alias } from '@ember/object/computed';
import { assert } from '@ember/debug';
import Evented from '@ember/object/evented';
import EmberObject, { computed } from '@ember/object';
import { assign } from '@ember/polyfills';
import queryString from 'npm:query-string';
import { task } from 'ember-concurrency';
import StreamLogger from 'nomad-ui/utils/classes/stream-logger';
import PollLogger from 'nomad-ui/utils/classes/poll-logger';

const MAX_OUTPUT_LENGTH = 50000;

export const fetchFailure = url => () => Ember.Logger.warn(`LOG FETCH: Couldn't connect to ${url}`);

const Log = EmberObject.extend(Evented, {
  // Parameters

  url: '',
  params: computed(() => ({})),
  logFetch() {
    assert('Log objects need a logFetch method, which should have an interface like window.fetch');
  },

  // Read-only state

  isStreaming: alias('logStreamer.poll.isRunning'),
  logPointer: null,
  logStreamer: null,

  // The top of the log
  head: '',

  // The bottom of the log
  tail: '',

  // The top or bottom of the log, depending on whether
  // the logPointer is pointed at head or tail
  output: computed('logPointer', 'head', 'tail', function() {
    return this.get('logPointer') === 'head' ? this.get('head') : this.get('tail');
  }),

  init() {
    this._super();

    const args = this.getProperties('url', 'params', 'logFetch');
    args.write = chunk => {
      let newTail = this.get('tail') + chunk;
      if (newTail.length > MAX_OUTPUT_LENGTH) {
        newTail = newTail.substr(newTail.length - MAX_OUTPUT_LENGTH);
      }
      this.set('tail', newTail);
      this.trigger('tick', chunk);
    };

    if (StreamLogger.isSupported) {
      this.set('logStreamer', StreamLogger.create(args));
    } else {
      this.set('logStreamer', PollLogger.create(args));
    }
  },

  destroy() {
    this.stop();
    this._super();
  },

  gotoHead: task(function*() {
    const logFetch = this.get('logFetch');
    const queryParams = queryString.stringify(
      assign(this.get('params'), {
        plain: true,
        origin: 'start',
        offset: 0,
      })
    );
    const url = `${this.get('url')}?${queryParams}`;

    this.stop();
    let text = yield logFetch(url).then(res => res.text(), fetchFailure(url));

    if (text && text.length > MAX_OUTPUT_LENGTH) {
      text = text.substr(0, MAX_OUTPUT_LENGTH);
      text += '\n\n---------- TRUNCATED: Click "tail" to view the bottom of the log ----------';
    }
    this.set('head', text);
    this.set('logPointer', 'head');
  }),

  gotoTail: task(function*() {
    const logFetch = this.get('logFetch');
    const queryParams = queryString.stringify(
      assign(this.get('params'), {
        plain: true,
        origin: 'end',
        offset: MAX_OUTPUT_LENGTH,
      })
    );
    const url = `${this.get('url')}?${queryParams}`;

    this.stop();
    let text = yield logFetch(url).then(res => res.text(), fetchFailure(url));

    this.set('tail', text);
    this.set('logPointer', 'tail');
  }),

  startStreaming() {
    this.set('logPointer', 'tail');
    return this.get('logStreamer').start();
  },

  stop() {
    this.get('logStreamer').stop();
  },
});

export default Log;

export function logger(urlProp, params, logFetch) {
  return computed(urlProp, params, function() {
    return Log.create({
      logFetch: logFetch.call(this),
      params: this.get(params),
      url: this.get(urlProp),
    });
  });
}
