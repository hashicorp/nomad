/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { scheduleOnce, once } from '@ember/runloop';
import { task } from 'ember-concurrency';
import WindowResizable from 'nomad-ui/mixins/window-resizable';
import {
  classNames,
  tagName,
  attributeBindings,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

const A_KEY = 65;

@classic
@tagName('pre')
@classNames('cli-window')
@attributeBindings('data-test-log-cli')
export default class StreamingFile extends Component.extend(WindowResizable) {
  'data-test-log-cli' = true;

  mode = 'streaming'; // head, tail, streaming
  isStreaming = true;
  logger = null;
  follow = true;
  shouldFillHeight = true;

  // Internal bookkeeping to avoid multiple scroll events on one frame
  requestFrame = true;

  didReceiveAttrs() {
    super.didReceiveAttrs();
    if (!this.logger) {
      return;
    }

    scheduleOnce('actions', this, this.performTask);
  }

  performTask() {
    switch (this.mode) {
      case 'head':
        this.set('follow', false);
        this.head.perform();
        break;
      case 'tail':
        this.set('follow', true);
        this.tail.perform();
        break;
      case 'streaming':
        this.set('follow', true);
        if (this.isStreaming) {
          this.stream.perform();
        } else {
          this.logger.stop();
        }
        break;
    }
  }

  scrollHandler() {
    const cli = this.element;

    // Scroll events can fire multiple times per frame, this eliminates
    // redundant computation.
    if (this.requestFrame) {
      window.requestAnimationFrame(() => {
        // If the scroll position is close enough to the bottom, autoscroll to the bottom
        this.set(
          'follow',
          cli.scrollHeight - cli.scrollTop - cli.clientHeight < 20
        );
        this.requestFrame = true;
      });
    }
    this.requestFrame = false;
  }

  keyDownHandler(e) {
    // Rebind select-all shortcut to only select the text in the
    // streaming file output.
    if ((e.metaKey || e.ctrlKey) && e.keyCode === A_KEY) {
      e.preventDefault();
      const selection = window.getSelection();
      selection.removeAllRanges();
      const range = document.createRange();
      range.selectNode(this.element);
      selection.addRange(range);
    }
  }

  didInsertElement() {
    super.didInsertElement(...arguments);
    if (this.shouldFillHeight) {
      this.fillAvailableHeight();
    }

    this.set('_scrollHandler', this.scrollHandler.bind(this));
    this.element.addEventListener('scroll', this._scrollHandler);

    this.set('_keyDownHandler', this.keyDownHandler.bind(this));
    document.addEventListener('keydown', this._keyDownHandler);
  }

  willDestroyElement() {
    super.willDestroyElement(...arguments);
    this.element.removeEventListener('scroll', this._scrollHandler);
    document.removeEventListener('keydown', this._keyDownHandler);
  }

  windowResizeHandler() {
    if (this.shouldFillHeight) {
      once(this, this.fillAvailableHeight);
    }
  }

  fillAvailableHeight() {
    // This math is arbitrary and far from bulletproof, but the UX
    // of having the log window fill available height is worth the hack.
    const margins = 30; // Account for padding and margin on either side of the CLI
    const cliWindow = this.element;
    cliWindow.style.height = `${
      window.innerHeight - cliWindow.offsetTop - margins
    }px`;
  }

  @task(function* () {
    yield this.get('logger.gotoHead').perform();
    scheduleOnce('afterRender', this, this.scrollToTop);
  })
  head;

  scrollToTop() {
    this.element.scrollTop = 0;
  }

  @task(function* () {
    yield this.get('logger.gotoTail').perform();
  })
  tail;

  synchronizeScrollPosition() {
    if (this.follow) {
      this.element.scrollTop = this.element.scrollHeight;
    }
  }

  @task(function* () {
    // Follow the log if the scroll position is near the bottom of the cli window
    this.logger.on('tick', this, 'scheduleScrollSynchronization');

    yield this.logger.startStreaming();
    this.logger.off('tick', this, 'scheduleScrollSynchronization');
  })
  stream;

  scheduleScrollSynchronization() {
    scheduleOnce('afterRender', this, this.synchronizeScrollPosition);
  }

  willDestroy() {
    super.willDestroy(...arguments);
    this.logger.stop();
  }
}
