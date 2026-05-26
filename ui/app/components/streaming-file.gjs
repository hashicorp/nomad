/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { scheduleOnce, once } from '@ember/runloop';
import { task } from 'ember-concurrency';
import { didInsert, didUpdate } from '@ember/render-modifiers';
import windowResize from 'nomad-ui/modifiers/window-resize';

const A_KEY = 65;

export default class StreamingFile extends Component {
  @tracked follow = true;

  requestFrame = true;
  cliElement = null;
  scrollHandlerBound = null;
  keyDownHandlerBound = null;

  get mode() {
    return this.args.mode ?? 'streaming';
  }

  get isStreaming() {
    return this.args.isStreaming ?? true;
  }

  get shouldFillHeight() {
    return this.args.shouldFillHeight ?? true;
  }

  setupElement = (element) => {
    this.cliElement = element;

    if (this.shouldFillHeight) {
      this.fillAvailableHeight();
    }

    this.scrollHandlerBound = this.scrollHandler.bind(this);
    this.cliElement.addEventListener('scroll', this.scrollHandlerBound);

    this.keyDownHandlerBound = this.keyDownHandler.bind(this);
    document.addEventListener('keydown', this.keyDownHandlerBound);
  };

  onArgsChange = () => {
    if (!this.args.logger) {
      return;
    }

    // Defer task start/stop so task state doesn't mutate during render.
    scheduleOnce('actions', this, this.performTask);
  };

  performTask = () => {
    switch (this.mode) {
      case 'head':
        this.follow = false;
        this.head.perform();
        break;
      case 'tail':
        this.follow = true;
        this.tail.perform();
        break;
      case 'streaming':
        this.follow = true;
        if (this.isStreaming) {
          this.stream.perform();
        } else {
          this.args.logger.stop();
        }
        break;
    }
  };

  scrollHandler() {
    const cli = this.cliElement;

    if (!cli) {
      return;
    }

    // Scroll events can fire multiple times per frame, this eliminates
    // redundant computation.
    if (this.requestFrame) {
      window.requestAnimationFrame(() => {
        // If the scroll position is close enough to the bottom, autoscroll to the bottom.
        this.follow = cli.scrollHeight - cli.scrollTop - cli.clientHeight < 20;
        this.requestFrame = true;
      });
    }

    this.requestFrame = false;
  }

  keyDownHandler(event) {
    // Rebind select-all shortcut to only select the text in the streaming file output.
    if ((event.metaKey || event.ctrlKey) && event.keyCode === A_KEY) {
      event.preventDefault();
      const selection = window.getSelection();
      selection.removeAllRanges();
      const range = document.createRange();
      range.selectNode(this.cliElement);
      selection.addRange(range);
    }
  }

  windowResizeHandler = () => {
    if (this.shouldFillHeight) {
      once(this, this.fillAvailableHeight);
    }
  };

  fillAvailableHeight = () => {
    if (!this.cliElement) {
      return;
    }

    // This math is arbitrary and far from bulletproof, but the UX of having
    // the log window fill available height is worth the hack.
    const margins = 30;
    this.cliElement.style.height = `${
      window.innerHeight - this.cliElement.offsetTop - margins
    }px`;
  };

  head = task(async () => {
    await this.args.logger.gotoHead.perform();
    scheduleOnce('afterRender', this, this.scrollToTop);
  });

  scrollToTop = () => {
    if (this.cliElement) {
      this.cliElement.scrollTop = 0;
    }
  };

  tail = task(async () => {
    await this.args.logger.gotoTail.perform();
  });

  synchronizeScrollPosition = () => {
    if (this.follow && this.cliElement) {
      this.cliElement.scrollTop = this.cliElement.scrollHeight;
    }
  };

  stream = task(async () => {
    // Follow the log if the scroll position is near the bottom of the cli window.
    this.args.logger.on('tick', this, 'scheduleScrollSynchronization');

    await this.args.logger.startStreaming();
    this.args.logger.off('tick', this, 'scheduleScrollSynchronization');
  });

  scheduleScrollSynchronization() {
    scheduleOnce('afterRender', this, this.synchronizeScrollPosition);
  }

  willDestroy() {
    super.willDestroy(...arguments);

    if (this.cliElement && this.scrollHandlerBound) {
      this.cliElement.removeEventListener('scroll', this.scrollHandlerBound);
    }

    if (this.keyDownHandlerBound) {
      document.removeEventListener('keydown', this.keyDownHandlerBound);
    }

    this.args.logger?.stop();
  }

  <template>
    <pre
      data-test-log-cli
      class="cli-window"
      {{didInsert this.setupElement}}
      {{didInsert this.onArgsChange}}
      {{didUpdate this.onArgsChange @logger this.mode this.isStreaming}}
      {{windowResize this.windowResizeHandler}}
      ...attributes
    >
      <code
        data-test-output
        class={{if @wrapped "wrapped"}}
      >{{@logger.output}}</code>
    </pre>
  </template>
}
