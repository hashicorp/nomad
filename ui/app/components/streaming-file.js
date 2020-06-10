import Component from '@ember/component';
import { run } from '@ember/runloop';
import { task } from 'ember-concurrency';
import WindowResizable from 'nomad-ui/mixins/window-resizable';
import { classNames, tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('pre')
@classNames('cli-window')
export default class StreamingFile extends Component.extend(WindowResizable) {
  'data-test-log-cli' = true;

  mode = 'streaming'; // head, tail, streaming
  isStreaming = true;
  logger = null;

  didReceiveAttrs() {
    if (!this.logger) {
      return;
    }

    run.scheduleOnce('actions', this, this.performTask);
  }

  performTask() {
    switch (this.mode) {
      case 'head':
        this.head.perform();
        break;
      case 'tail':
        this.tail.perform();
        break;
      case 'streaming':
        if (this.isStreaming) {
          this.stream.perform();
        } else {
          this.logger.stop();
        }
        break;
    }
  }

  didInsertElement() {
    this.fillAvailableHeight();
  }

  windowResizeHandler() {
    run.once(this, this.fillAvailableHeight);
  }

  fillAvailableHeight() {
    // This math is arbitrary and far from bulletproof, but the UX
    // of having the log window fill available height is worth the hack.
    const margins = 30; // Account for padding and margin on either side of the CLI
    const cliWindow = this.element;
    cliWindow.style.height = `${window.innerHeight - cliWindow.offsetTop - margins}px`;
  }

  @task(function*() {
    yield this.get('logger.gotoHead').perform();
    run.scheduleOnce('afterRender', this, this.scrollToTop);
  })
  head;

  scrollToTop() {
    this.element.scrollTop = 0;
  }

  @task(function*() {
    yield this.get('logger.gotoTail').perform();
    run.scheduleOnce('afterRender', this, this.synchronizeScrollPosition, [true]);
  })
  tail;

  synchronizeScrollPosition(force = false) {
    const cliWindow = this.element;
    if (cliWindow.scrollHeight - cliWindow.scrollTop < 10 || force) {
      // If the window is approximately scrolled to the bottom, follow the log
      cliWindow.scrollTop = cliWindow.scrollHeight;
    }
  }

  @task(function*() {
    // Force the scroll position to the bottom of the window when starting streaming
    this.logger.one('tick', () => {
      run.scheduleOnce('afterRender', this, this.synchronizeScrollPosition, [true]);
    });

    // Follow the log if the scroll position is near the bottom of the cli window
    this.logger.on('tick', this, 'scheduleScrollSynchronization');

    yield this.logger.startStreaming();
    this.logger.off('tick', this, 'scheduleScrollSynchronization');
  })
  stream;

  scheduleScrollSynchronization() {
    run.scheduleOnce('afterRender', this, this.synchronizeScrollPosition);
  }

  willDestroy() {
    this.logger.stop();
  }
}
