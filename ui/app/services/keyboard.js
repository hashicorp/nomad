import Service from '@ember/service';
import { inject as service } from '@ember/service';
import { timeout, restartableTask } from 'ember-concurrency';
import { tracked } from '@glimmer/tracking';
import { compare } from '@ember/utils';
import { A } from '@ember/array';
import EmberRouter from '@ember/routing/router';

const DEBOUNCE_MS = 750;

export default class KeyboardService extends Service {
  /**
   * @type {EmberRouter}
   */
  @service router;

  keyCommands = [
    {
      label: 'Konami',
      pattern: [
        'ArrowUp',
        'ArrowUp',
        'ArrowDown',
        'ArrowDown',
        'ArrowLeft',
        'ArrowRight',
        'ArrowLeft',
        'ArrowRight',
        'b',
        'a',
      ],
    },
    {
      label: 'Go to Jobs',
      pattern: ['g', 'j'],
      action: () => this.router.transitionTo('jobs'),
    },
    {
      label: 'Go to Storage',
      pattern: ['g', 's', 't'],
      action: () => this.router.transitionTo('csi.volumes'),
    },
    {
      label: 'Go to Servers',
      pattern: ['g', 's', 'e'],
      action: () => this.router.transitionTo('servers'),
    },
    {
      label: 'Go to Clients',
      pattern: ['g', 'c'],
      action: () => this.router.transitionTo('clients'),
    },
    {
      label: 'Go to Topology',
      pattern: ['g', 't'],
      action: () => this.router.transitionTo('topology'),
    },
    {
      label: 'Go to Evaluations',
      pattern: ['g', 'e'],
      action: () => this.router.transitionTo('evaluations'),
    },
    {
      label: 'Go to ACL Tokens',
      pattern: ['g', 'a'],
      action: () => this.router.transitionTo('settings.tokens'),
    },
  ];

  @tracked buffer = A([]);

  /**
   *
   * @param {KeyboardEvent} event
   */
  recordKeypress(event) {
    const inputElements = ['input', 'textarea'];
    const targetElementName = event.target.nodeName.toLowerCase();
    // Don't fire keypress events from within an input field
    if (!inputElements.includes(targetElementName)) {
      const { key } = event;
      const shifted = event.getModifierState('Shift');
      this.addKeyToBuffer.perform(key);
    }
  }

  /**
   *
   * @param {KeyboardEvent} key
   */
  @restartableTask *addKeyToBuffer(key) {
    this.buffer.pushObject(key);
    yield timeout(DEBOUNCE_MS);
    this.clearBuffer();
  }

  get matchedCommand() {
    // Ember Compare: returns 0 if there's no diff between arrays.
    // TODO: do we think this is faster than a pure JS .join("") comparison?
    const match = this.keyCommands.find(
      (command) => !compare(command.pattern, this.buffer)
    );
    if (match) {
      console.log('Performing Action:', match.label);
      match.action();
    }
    return match;
  }

  clearBuffer() {
    this.buffer.clear();
  }

  listenForKeypress() {
    document.addEventListener('keydown', this.recordKeypress.bind(this));
  }
}
