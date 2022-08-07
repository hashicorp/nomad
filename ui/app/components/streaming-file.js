import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { htmlSafe } from '@ember/template';
import { tracked } from '@glimmer/tracking';

import Component from '@ember/component';
import { scheduleOnce, once } from '@ember/runloop';
import { task, timeout } from 'ember-concurrency';
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
    this.fillAvailableHeight();

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
    once(this, this.fillAvailableHeight);
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
    this.logger.on('tick', this, () => {
      console.log('tick');
      if (this.activeFilterBuffer) {
        console.log('ya', this.activeFilterBuffer);
        let filteredOutput = this.logger.output.string
          .split('\n')
          .filter((line) => line.includes(this.activeFilterBuffer))
          .join('\n');
        console.log({ filteredOutput });
        if (filteredOutput) {
          this.filteredOutput = htmlSafe(filteredOutput);
        }
      } else {
        this.filteredOutput = '';
      }
    });

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

  //#region Keynav Demo
  @service keyboard;

  // @tracked
  // activeFilterBuffer = "";

  @tracked listenForBuffer = false;
  @tracked activeFilterBuffer = '';

  // @computed('keyboard.buffer', 'listenForBuffer')
  @(task(function* () {
    do {
      if (this.keyboard.buffer.length) {
        console.log('doin', this.keyboard.buffer);
        this.activeFilterBuffer = this.keyboard.buffer.join('');
        yield this.keyboard.buffer.join('');
      }
      yield timeout(500);
    } while (this.keyboard.buffer.length);
  }).drop())
  filterBufferWatcher;

  // get filterBufferWatcher() {
  //   if (this.listenForBuffer && this.keyboard.buffer.length) {
  //     console.log("buffer???", this.keyboard.buffer);
  //     this.activeFilterBuffer = this.keyboard.buffer.join();
  //   }
  //   else {
  //     console.log("nope");
  //     // return this.activeFilterBuffer;
  //   }
  // }

  @tracked
  filteredOutput = '';

  @action
  filterLogs() {
    console.log('filtering logs, what is keyboard buffer?');
    // this.activeFilterBuffer = "lol";
    // this.listenForBuffer = true;
    this.filteredOutput = 'filtering...';
    this.filterBufferWatcher.perform();
    console.log('buffy', this.filterBufferWatcher);
    // setTimeout(() => this.listenForBuffer = false, 3000); // TODO: ember concurrency
  }

  @action
  unFilterLogs() {
    this.activeFilterBuffer = '';
    // this.listenForBuffer = false;
  }

  keyCommands = [
    {
      label: 'Filter Logs',
      pattern: ['f'],
      action: () => this.filterLogs(),
    },
    {
      label: 'Nuke log filter',
      pattern: ['Escape'],
      action: () => this.unFilterLogs(),
    },
  ];

  //#endregion Keynav Demo
}
