import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { computed } from '@ember/object';
import { gt } from '@ember/object/computed';
import { equal } from '@ember/object/computed';
import RSVP from 'rsvp';
import Log from 'nomad-ui/utils/classes/log';
import timeout from 'nomad-ui/utils/timeout';

export default Component.extend({
  token: service(),

  classNames: ['boxed-section', 'task-log'],

  'data-test-file-viewer': true,

  allocation: null,
  task: null,
  file: null,
  stat: null, // { Name, IsDir, Size, FileMode, ModTime, ContentType }

  // When true, request logs from the server agent
  useServer: false,

  // When true, logs cannot be fetched from either the client or the server
  noConnection: false,

  clientTimeout: 1000,
  serverTimeout: 5000,

  mode: 'head',

  fileComponent: computed('stat.ContentType', function() {
    const contentType = this.stat.ContentType || '';

    if (contentType.startsWith('image/')) {
      return 'image';
    } else if (contentType.startsWith('text/') || contentType.startsWith('application/json')) {
      return 'stream';
    } else {
      return 'unknown';
    }
  }),

  isLarge: gt('stat.Size', 50000),

  fileTypeIsUnknown: equal('fileComponent', 'unknown'),
  isStreamable: equal('fileComponent', 'stream'),
  isStreaming: false,

  catUrl: computed('allocation.id', 'task.name', 'file', function() {
    const encodedPath = encodeURIComponent(`${this.task.name}/${this.file}`);
    return `/v1/client/fs/cat/${this.allocation.id}?path=${encodedPath}`;
  }),

  fetchMode: computed('isLarge', 'mode', function() {
    if (this.mode === 'streaming') {
      return 'stream';
    }

    if (!this.isLarge) {
      return 'cat';
    } else if (this.mode === 'head' || this.mode === 'tail') {
      return 'readat';
    }
  }),

  fileUrl: computed(
    'allocation.id',
    'allocation.node.httpAddr',
    'fetchMode',
    'useServer',
    function() {
      const address = this.get('allocation.node.httpAddr');
      const url = `/v1/client/fs/${this.fetchMode}/${this.allocation.id}`;
      return this.useServer ? url : `//${address}${url}`;
    }
  ),

  fileParams: computed('task.name', 'file', 'mode', function() {
    // The Log class handles encoding query params
    const path = `${this.task.name}/${this.file}`;

    switch (this.mode) {
      case 'head':
        return { path, offset: 0, limit: 50000 };
      case 'tail':
        return { path, offset: this.stat.Size - 50000, limit: 50000 };
      case 'streaming':
        return { path, offset: 50000, origin: 'end' };
      default:
        return { path };
    }
  }),

  logger: computed('fileUrl', 'fileParams', 'mode', function() {
    // The cat and readat APIs are in plainText while the stream API is always encoded.
    const plainText = this.mode === 'head' || this.mode === 'tail';

    // If the file request can't settle in one second, the client
    // must be unavailable and the server should be used instead
    const timing = this.useServer ? this.serverTimeout : this.clientTimeout;
    const logFetch = url =>
      RSVP.race([this.token.authorizedRequest(url), timeout(timing)]).then(
        response => {
          if (!response || !response.ok) {
            this.nextErrorState(response);
          }
          return response;
        },
        error => this.nextErrorState(error)
      );

    return Log.create({
      logFetch,
      plainText,
      params: this.fileParams,
      url: this.fileUrl,
    });
  }),

  nextErrorState(error) {
    if (this.useServer) {
      this.set('noConnection', true);
    } else {
      this.send('failoverToServer');
    }
    throw error;
  },

  actions: {
    toggleStream() {
      this.set('mode', 'streaming');
      this.toggleProperty('isStreaming');
    },
    gotoHead() {
      this.set('mode', 'head');
      this.set('isStreaming', false);
    },
    gotoTail() {
      this.set('mode', 'tail');
      this.set('isStreaming', false);
    },
    failoverToServer() {
      this.set('useServer', true);
    },
  },
});
