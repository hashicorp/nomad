/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Ember from 'ember';
import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { action, computed } from '@ember/object';
import { equal, gt } from '@ember/object/computed';
import RSVP from 'rsvp';
import Log from 'nomad-ui/utils/classes/log';
import timeout from 'nomad-ui/utils/timeout';
import { classNames, attributeBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('boxed-section', 'task-log')
@attributeBindings('data-test-file-viewer')
export default class File extends Component {
  @service token;
  @service system;

  'data-test-file-viewer' = true;

  allocation = null;
  taskState = null;
  file = null;
  stat = null; // { Name, IsDir, Size, FileMode, ModTime, ContentType }

  // When true, request logs from the server agent
  useServer = false;

  // When true, logs cannot be fetched from either the client or the server
  noConnection = false;

  clientTimeout = 1000;
  serverTimeout = 5000;

  mode = 'head';

  @computed('stat.ContentType')
  get fileComponent() {
    const contentType = this.stat.ContentType || '';

    if (contentType.startsWith('image/')) {
      return 'image';
    } else if (
      contentType.startsWith('text/') ||
      contentType.startsWith('application/json')
    ) {
      return 'stream';
    } else {
      return 'unknown';
    }
  }

  @gt('stat.Size', 50000) isLarge;

  @equal('fileComponent', 'unknown') fileTypeIsUnknown;
  @equal('fileComponent', 'stream') isStreamable;
  isStreaming = false;

  @computed('allocation.id', 'taskState.name', 'file')
  get catUrlWithoutRegion() {
    const taskUrlPrefix = this.taskState ? `${this.taskState.name}/` : '';
    const encodedPath = encodeURIComponent(`${taskUrlPrefix}${this.file}`);
    return `/v1/client/fs/cat/${this.allocation.id}?path=${encodedPath}`;
  }

  @computed('catUrlWithoutRegion', 'system.{activeRegion,shouldIncludeRegion}')
  get catUrl() {
    let apiPath = this.catUrlWithoutRegion;
    if (this.system.shouldIncludeRegion) {
      apiPath += `&region=${this.system.activeRegion}`;
    }
    return apiPath;
  }

  @computed('isLarge', 'mode')
  get fetchMode() {
    if (this.mode === 'streaming') {
      return 'stream';
    }

    if (!this.isLarge) {
      return 'cat';
    } else if (this.mode === 'head' || this.mode === 'tail') {
      return 'readat';
    }

    return undefined;
  }

  @computed('allocation.{id,node.httpAddr}', 'fetchMode', 'useServer')
  get fileUrl() {
    const address = this.get('allocation.node.httpAddr');
    const url = `/v1/client/fs/${this.fetchMode}/${this.allocation.id}`;
    return this.useServer ? url : `//${address}${url}`;
  }

  @computed('file', 'mode', 'stat.Size', 'taskState.name')
  get fileParams() {
    // The Log class handles encoding query params
    const taskUrlPrefix = this.taskState ? `${this.taskState.name}/` : '';
    const path = `${taskUrlPrefix}${this.file}`;

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
  }

  @computed(
    'clientTimeout',
    'fileParams',
    'fileUrl',
    'mode',
    'serverTimeout',
    'useServer'
  )
  get logger() {
    // The cat and readat APIs are in plainText while the stream API is always encoded.
    const plainText = this.mode === 'head' || this.mode === 'tail';

    // If the file request can't settle in one second, the client
    // must be unavailable and the server should be used instead
    const timing = this.useServer ? this.serverTimeout : this.clientTimeout;
    const logFetch = (url) =>
      RSVP.race([this.token.authorizedRequest(url), timeout(timing)]).then(
        (response) => {
          if (!response || !response.ok) {
            this.nextErrorState(response);
          }
          return response;
        },
        (error) => this.nextErrorState(error)
      );

    return Log.create({
      logFetch,
      plainText,
      params: this.fileParams,
      url: this.fileUrl,
    });
  }

  nextErrorState(error) {
    if (this.useServer) {
      this.set('noConnection', true);
    } else {
      this.send('failoverToServer');
    }
    throw error;
  }

  @action
  toggleStream() {
    this.set('mode', 'streaming');
    this.toggleProperty('isStreaming');
  }

  @action
  gotoHead() {
    this.set('mode', 'head');
    this.set('isStreaming', false);
  }

  @action
  gotoTail() {
    this.set('mode', 'tail');
    this.set('isStreaming', false);
  }

  @action
  failoverToServer() {
    this.set('useServer', true);
  }

  @action
  async downloadFile() {
    const timing = this.useServer ? this.serverTimeout : this.clientTimeout;

    try {
      const response = await RSVP.race([
        this.token.authorizedRequest(this.catUrlWithoutRegion),
        timeout(timing),
      ]);

      if (!response || !response.ok) throw new Error('file download timeout');

      // Don't download in tests. Unfortunately, since the download is triggered
      // by the download attribute of the ephemeral anchor element, there's no
      // way to stub this in tests.
      if (Ember.testing) return;

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const downloadAnchor = document.createElement('a');

      downloadAnchor.href = url;
      downloadAnchor.target = '_blank';
      downloadAnchor.rel = 'noopener noreferrer';
      downloadAnchor.download = this.file;

      // Appending the element to the DOM is required for Firefox support
      document.body.appendChild(downloadAnchor);
      downloadAnchor.click();
      downloadAnchor.remove();

      window.URL.revokeObjectURL(url);
    } catch (err) {
      this.nextErrorState(err);
    }
  }
}
