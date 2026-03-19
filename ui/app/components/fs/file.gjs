/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { service } from '@ember/service';
import { macroCondition, isTesting } from '@embroider/macros';
import { and } from 'ember-truth-helpers';
import { eq } from 'ember-truth-helpers';
import { on } from '@ember/modifier';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import didUpdate from '@ember/render-modifiers/modifiers/did-update';
import RSVP from 'rsvp';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import ImageFile from 'nomad-ui/components/image-file';
import StreamingFile from 'nomad-ui/components/streaming-file';
import Log from 'nomad-ui/utils/classes/log';
import timeout from 'nomad-ui/utils/timeout';

export default class File extends Component {
  @service token;
  @service system;

  @tracked useServer = false;
  @tracked noConnection = false;
  @tracked mode = 'head';
  @tracked isStreaming = false;
  @tracked logger = null;

  clientTimeout = 1000;
  serverTimeout = 5000;

  get fileComponent() {
    const contentType = this.args.stat?.ContentType || '';

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

  get isLarge() {
    return this.args.stat?.Size > 50000;
  }

  get fileTypeIsUnknown() {
    return this.fileComponent === 'unknown';
  }

  get isStreamable() {
    return this.fileComponent === 'stream';
  }

  get catUrlWithoutRegion() {
    const taskUrlPrefix = this.args.taskState ? `${this.args.taskState.name}/` : '';
    const encodedPath = encodeURIComponent(`${taskUrlPrefix}${this.args.file}`);
    return `/v1/client/fs/cat/${this.args.allocation.id}?path=${encodedPath}`;
  }

  get catUrl() {
    let apiPath = this.catUrlWithoutRegion;
    const activeRegion = this.system.activeRegion;

    if (this.system.shouldIncludeRegion && activeRegion) {
      apiPath += `&region=${activeRegion}`;
    }
    return apiPath;
  }

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

  get fileUrl() {
    const address = this.args.allocation?.node?.httpAddr;
    const url = `/v1/client/fs/${this.fetchMode}/${this.args.allocation.id}`;
    return this.useServer ? url : `//${address}${url}`;
  }

  get fileParams() {
    const taskUrlPrefix = this.args.taskState ? `${this.args.taskState.name}/` : '';
    const path = `${taskUrlPrefix}${this.args.file}`;

    switch (this.mode) {
      case 'head':
        return { path, offset: 0, limit: 50000 };
      case 'tail':
        return { path, offset: this.args.stat.Size - 50000, limit: 50000 };
      case 'streaming':
        return { path, offset: 50000, origin: 'end' };
      default:
        return { path };
    }
  }

  buildLogger = () => {
    const plainText = this.mode === 'head' || this.mode === 'tail';
    const timing = this.useServer ? this.serverTimeout : this.clientTimeout;
    const logFetch = (url) =>
      RSVP.race([this.token.authorizedRequest(url), timeout(timing)]).then(
        (response) => {
          if (!response || !response.ok) {
            this.nextErrorState(response);
          }
          return response;
        },
        (error) => this.nextErrorState(error),
      );

    return Log.create({
      logFetch,
      plainText,
      params: this.fileParams,
      url: this.fileUrl,
    });
  };

  refreshLogger = () => {
    this.logger?.stop();
    this.logger = this.buildLogger();
  };

  nextErrorState(error) {
    if (this.useServer) {
      this.noConnection = true;
    } else {
      this.failoverToServer();
    }
    throw error;
  }

  toggleStream = () => {
    this.mode = 'streaming';
    this.isStreaming = !this.isStreaming;
    this.refreshLogger();
  };

  gotoHead = () => {
    this.mode = 'head';
    this.isStreaming = false;
    this.refreshLogger();
  };

  gotoTail = () => {
    this.mode = 'tail';
    this.isStreaming = false;
    this.refreshLogger();
  };

  failoverToServer = () => {
    this.useServer = true;
    this.refreshLogger();
  };

  downloadFile = async () => {
    const timing = this.useServer ? this.serverTimeout : this.clientTimeout;

    try {
      const response = await RSVP.race([
        this.token.authorizedRequest(this.catUrl),
        timeout(timing),
      ]);

      if (!response || !response.ok) throw new Error('file download timeout');

      if (macroCondition(isTesting())) return;

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const downloadAnchor = document.createElement('a');

      downloadAnchor.href = url;
      downloadAnchor.target = '_blank';
      downloadAnchor.rel = 'noopener noreferrer';
      downloadAnchor.download = this.args.file;

      document.body.appendChild(downloadAnchor);
      downloadAnchor.click();
      downloadAnchor.remove();

      window.URL.revokeObjectURL(url);
    } catch (err) {
      this.nextErrorState(err);
    }
  };

  willDestroy() {
    super.willDestroy(...arguments);
    this.logger?.stop();
  }

  <template>
    <div
      class="boxed-section task-log"
      data-test-file-viewer
      ...attributes
      {{didInsert this.refreshLogger}}
      {{didUpdate
        this.refreshLogger
        @allocation.id
        @allocation.node.httpAddr
        @taskState.name
        @file
        @stat.Size
      }}
    >
      {{#if this.noConnection}}
        <div data-test-connection-error class="notification is-error">
          <h3 class="title is-4">Cannot fetch file</h3>
          <p>The files for this {{if @taskState "task" "allocation"}} are inaccessible. Check the condition of the client the allocation is on.</p>
        </div>
      {{/if}}
      <div data-test-header class="boxed-section-head">
        {{yield}}
        <span class="pull-right">

          {{#unless this.fileTypeIsUnknown}}
            <button
              data-test-log-action="raw"
              class="button is-white is-compact"
              {{on "click" this.downloadFile}}
              type="button"
            >View Raw File</button>
          {{/unless}}
          {{#if (and this.isLarge this.isStreamable)}}
            <button
              data-test-log-action="head"
              class="button is-white is-compact"
              {{on "click" this.gotoHead}}
              type="button"
            >Head</button>
            <button
              data-test-log-action="tail"
              class="button is-white is-compact"
              {{on "click" this.gotoTail}}
              type="button"
            >Tail</button>
          {{/if}}
          {{#if this.isStreamable}}
            <button
              data-test-log-action="toggle-stream"
              class="button is-white is-compact"
              {{on "click" this.toggleStream}}
              type="button"
              title="{{if this.logger.isStreaming 'Pause' 'Start'}} streaming"
            >
              <HdsIcon
                @name={{if this.logger.isStreaming "pause" "play"}}
                @isInline={{true}}
              />
            </button>
          {{/if}}
        </span>
      </div>
      <div
        data-test-file-box
        class="boxed-section-body {{if (eq this.fileComponent 'stream') 'is-dark is-full-bleed'}}"
      >
        {{#if (eq this.fileComponent "stream")}}
          <StreamingFile
            @logger={{this.logger}}
            @mode={{this.mode}}
            @isStreaming={{this.isStreaming}}
          />
        {{else if (eq this.fileComponent "image")}}
          <ImageFile
            @src={{this.catUrl}}
            @alt={{@stat.Name}}
            @size={{@stat.Size}}
          />
        {{else}}
          <div data-test-unsupported-type class="empty-message is-hollow">
            <h3 class="empty-message-headline">Unsupported File Type</h3>
            <p class="empty-message-body message">The Nomad UI could not render this file, but you can still view the file directly.</p>
            <p class="empty-message-body">
              <button
                data-test-log-action="raw"
                class="button is-light"
                {{on "click" this.downloadFile}}
                type="button"
              >View Raw File</button>
            </p>
          </div>
        {{/if}}
      </div>
    </div>
  </template>
}