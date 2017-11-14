import Ember from 'ember';
import queryString from 'npm:query-string';
import { task } from 'ember-concurrency';
import StreamLogger from 'nomad-ui/utils/classes/stream-logger';
import PollLogger from 'nomad-ui/utils/classes/poll-logger';

const { Object: EmberObject, Evented, computed, assign } = Ember;

const Log = EmberObject.extend(Evented, {
  // Parameters

  url: '',
  params: computed(() => ({})),
  logFetch() {
    Ember.assert(
      'Log objects need a logFetch method, which should have an interface like window.fetch'
    );
  },

  // Read-only state

  isStreaming: computed.alias('logStreamer.poll.isRunning'),
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
      if (newTail.length > 50000) {
        newTail = newTail.substr(newTail.length - 50000);
      }
      this.set('tail', newTail);
      this.trigger('tick');
    };

    if (window.ReadableStream) {
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
    let text = yield logFetch(url).then(res => res.text());

    if (text.length > 50000) {
      text = text.substr(0, 50000);
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
        offset: 50000,
      })
    );
    const url = `${this.get('url')}?${queryParams}`;

    this.stop();
    let text = yield logFetch(url).then(res => res.text());

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
