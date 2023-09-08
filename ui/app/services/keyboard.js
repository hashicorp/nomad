/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Service from '@ember/service';
import { inject as service } from '@ember/service';
import { timeout, restartableTask } from 'ember-concurrency';
import { tracked } from '@glimmer/tracking';
import { compare } from '@ember/utils';
import { A } from '@ember/array';
// eslint-disable-next-line no-unused-vars
import EmberRouter from '@ember/routing/router';
import { schedule } from '@ember/runloop';
import { action, set } from '@ember/object';
import { guidFor } from '@ember/object/internals';
import { assert } from '@ember/debug';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

/**
 * @typedef {Object} KeyCommand
 * @property {string} label
 * @property {string[]} pattern
 * @property {any} action
 * @property {boolean} [requireModifier]
 * @property {boolean} [enumerated]
 * @property {boolean} [recording]
 * @property {boolean} [custom]
 * @property {boolean} [exclusive]
 */

const DEBOUNCE_MS = 750;
// This modifies event.key to a symbol; get the digit equivalent to perform commands
const DIGIT_MAP = {
  '!': 1,
  '@': 2,
  '#': 3,
  $: 4,
  '%': 5,
  '^': 6,
  '&': 7,
  '*': 8,
  '(': 9,
  ')': 0,
};

const DISALLOWED_KEYS = [
  'Shift',
  'Backspace',
  'Delete',
  'Meta',
  'Alt',
  'Control',
  'Tab',
  'CapsLock',
  'Clear',
  'ScrollLock',
];

export default class KeyboardService extends Service {
  /**
   * @type {EmberRouter}
   */
  @service router;

  @service config;

  @tracked shortcutsVisible = false;
  @tracked buffer = A([]);
  @tracked displayHints = false;

  @localStorageProperty('keyboardNavEnabled', true) enabled;

  defaultPatterns = {
    'Go to Jobs': ['g', 'j'],
    'Go to Storage': ['g', 'r'],
    'Go to Variables': ['g', 'v'],
    'Go to Servers': ['g', 's'],
    'Go to Clients': ['g', 'c'],
    'Go to Topology': ['g', 't'],
    'Go to Evaluations': ['g', 'e'],
    'Go to Profile': ['g', 'p'],
    'Next Subnav': ['Shift+ArrowRight'],
    'Previous Subnav': ['Shift+ArrowLeft'],
    'Previous Main Section': ['Shift+ArrowUp'],
    'Next Main Section': ['Shift+ArrowDown'],
    'Show Keyboard Shortcuts': ['Shift+?'],
  };

  /**
   * @type {MutableArray<KeyCommand>}
   */
  @tracked
  keyCommands = A(
    [
      {
        label: 'Go to Jobs',
        action: () => this.router.transitionTo('jobs'),
        rebindable: true,
      },
      {
        label: 'Go to Storage',
        action: () => this.router.transitionTo('csi.volumes'),
        rebindable: true,
      },
      {
        label: 'Go to Variables',
        action: () => this.router.transitionTo('variables'),
      },
      {
        label: 'Go to Servers',
        action: () => this.router.transitionTo('servers'),
        rebindable: true,
      },
      {
        label: 'Go to Clients',
        action: () => this.router.transitionTo('clients'),
        rebindable: true,
      },
      {
        label: 'Go to Topology',
        action: () => this.router.transitionTo('topology'),
        rebindable: true,
      },
      {
        label: 'Go to Evaluations',
        action: () => this.router.transitionTo('evaluations'),
        rebindable: true,
      },
      {
        label: 'Go to Profile',
        action: () => this.router.transitionTo('settings.tokens'),
        rebindable: true,
      },
      {
        label: 'Next Subnav',
        action: () => {
          this.traverseLinkList(this.subnavLinks, 1);
        },
        requireModifier: true,
        rebindable: true,
      },
      {
        label: 'Previous Subnav',
        action: () => {
          this.traverseLinkList(this.subnavLinks, -1);
        },
        requireModifier: true,
        rebindable: true,
      },
      {
        label: 'Previous Main Section',
        action: () => {
          this.traverseLinkList(this.navLinks, -1);
        },
        requireModifier: true,
        rebindable: true,
      },
      {
        label: 'Next Main Section',
        action: () => {
          this.traverseLinkList(this.navLinks, 1);
        },
        requireModifier: true,
        rebindable: true,
      },
      {
        label: 'Show Keyboard Shortcuts',
        action: () => {
          this.shortcutsVisible = true;
        },
      },
    ].map((command) => {
      const persistedValue = window.localStorage.getItem(
        `keyboard.command.${command.label}`
      );
      if (persistedValue) {
        set(command, 'pattern', JSON.parse(persistedValue));
        set(command, 'custom', true);
      } else {
        set(command, 'pattern', this.defaultPatterns[command.label]);
      }
      return command;
    })
  );

  /**
   * For Dynamic/iterative keyboard shortcuts, we want to do a couple things to make them more human-friendly:
   * 1. Make them 1-based, instead of 0-based
   * 2. Prefix numbers 1-9 with "0" to make it so "Shift+10" doesn't trigger "Shift+1" then "0", etc.
   * ^--- stops being a good solution with 100+ row lists/tables, but a better UX than waiting for shift key-up otherwise
   *
   * @param {number} iter
   * @returns {string[]}
   */
  cleanPattern(iter) {
    iter = iter + 1; // first item should be Shift+1, not Shift+0
    assert('Dynamic keyboard shortcuts only work up to 99 digits', iter < 100);
    return [`Shift+${('0' + iter).slice(-2)}`]; // Shift+01, not Shift+1
  }

  recomputeEnumeratedCommands() {
    this.keyCommands.filterBy('enumerated').forEach((command, iter) => {
      command.pattern = this.cleanPattern(iter);
    });
  }

  addCommands(commands) {
    schedule('afterRender', () => {
      commands.forEach((command) => {
        if (command.exclusive) {
          this.removeCommands(
            this.keyCommands.filterBy('label', command.label)
          );
        }
        this.keyCommands.pushObject(command);
        if (command.enumerated) {
          // Recompute enumerated numbers to handle things like sort
          this.recomputeEnumeratedCommands();
        }
      });
    });
  }

  removeCommands(commands = A([])) {
    this.keyCommands.removeObjects(commands);
  }

  //#region Nav Traversal

  subnavLinks = [];
  navLinks = [];

  /**
   * Map over a passed element's links and determine if they're routable
   * If so, return them in a transitionTo-able format
   *
   * @param {HTMLElement} element did-insertable menu container element
   * @param {Object} args
   * @param {('main' | 'subnav')} args.type determine which traversable list the routes belong to
   */
  @action
  registerNav(element, _, args) {
    const { type } = args;
    const links = Array.from(element.querySelectorAll('a:not(.loading)'))
      .map((link) => {
        if (link.getAttribute('href')) {
          return {
            route: this.router.recognize(link.getAttribute('href'))?.name,
            parent: guidFor(element),
          };
        }
      })
      .compact();

    if (type === 'main') {
      this.navLinks = links;
    } else if (type === 'subnav') {
      this.subnavLinks = links;
    }
  }

  /**
   * Removes links associated with a specific nav.
   * guidFor is necessary because willDestroy runs async;
   * it can happen after the next page's did-insert, so we .reject() instead of resetting to [].
   *
   * @param {HTMLElement} element
   */
  @action
  unregisterSubnav(element) {
    this.subnavLinks = this.subnavLinks.reject(
      (link) => link.parent === guidFor(element)
    );
  }

  /**
   *
   * @param {Array<string>} links - array of root.branch.twig strings
   * @param {number} traverseBy - positive or negative number to move along links
   */
  traverseLinkList(links, traverseBy) {
    // afterRender because LinkTos evaluate their href value at render time
    schedule('afterRender', () => {
      if (links.length) {
        let activeLink = links.find((link) => this.router.isActive(link.route));

        // If no activeLink, means we're nested within a primary section.
        // Luckily, Ember's RouteInfo.find() gives us access to parents and connected leaves of a route.
        // So, if we're on /csi/volumes but the nav link is to /csi, we'll .find() it.
        // Similarly, /job/:job/taskgroupid/index will find /job.
        if (!activeLink) {
          activeLink = links.find((link) => {
            return this.router.currentRoute.find((r) => {
              return r.name === link.route || `${r.name}.index` === link.route;
            });
          });
        }

        if (activeLink) {
          const activeLinkPosition = links.indexOf(activeLink);
          const nextPosition = activeLinkPosition + traverseBy;

          // Modulo (%) logic: if the next position is longer than the array, wrap to 0.
          // If it's before the beginning, wrap to the end.
          const nextLink =
            links[((nextPosition % links.length) + links.length) % links.length]
              .route;

          this.router.transitionTo(nextLink);
        }
      }
    });
  }

  //#endregion Nav Traversal

  /**
   *
   * @param {("press" | "release")} type
   * @param {KeyboardEvent} event
   */
  recordKeypress(type, event) {
    const inputElements = ['input', 'textarea', 'code'];
    const disallowedClassNames = [
      'ember-basic-dropdown-trigger',
      'dropdown-option',
    ];
    const targetElementName = event.target.nodeName.toLowerCase();
    const inputDisallowed =
      inputElements.includes(targetElementName) ||
      disallowedClassNames.any((className) =>
        event.target.classList.contains(className)
      );

    // Don't fire keypress events from within an input field
    if (!inputDisallowed) {
      // Treat Shift like a special modifier key.
      // If it's depressed, display shortcuts
      const { key } = event;
      const shifted = event.getModifierState('Shift');
      if (type === 'press') {
        if (key === 'Shift') {
          this.displayHints = true;
        } else {
          if (!DISALLOWED_KEYS.includes(key)) {
            this.addKeyToBuffer.perform(key, shifted, event);
          }
        }
      } else if (type === 'release') {
        if (key === 'Shift') {
          this.displayHints = false;
        }
      }
    }
  }

  rebindCommand = (cmd, ele) => {
    ele.target.blur(); // keynav ignores on inputs
    this.clearBuffer();
    set(cmd, 'recording', true);
    set(cmd, 'previousPattern', cmd.pattern);
    set(cmd, 'pattern', null);
  };

  endRebind = (cmd) => {
    set(cmd, 'custom', true);
    set(cmd, 'recording', false);
    set(cmd, 'previousPattern', null);
    window.localStorage.setItem(
      `keyboard.command.${cmd.label}`,
      JSON.stringify([...this.buffer])
    );
  };

  resetCommandToDefault = (cmd) => {
    window.localStorage.removeItem(`keyboard.command.${cmd.label}`);
    set(cmd, 'pattern', this.defaultPatterns[cmd.label]);
    set(cmd, 'custom', false);
  };

  /**
   *
   * @param {string} key
   * @param {boolean} shifted
   */
  @restartableTask *addKeyToBuffer(key, shifted, event) {
    // Replace key with its unshifted equivalent if it's a number key
    if (shifted && key in DIGIT_MAP) {
      key = DIGIT_MAP[key];
    }
    this.buffer.pushObject(shifted ? `Shift+${key}` : key);
    let recorder = this.keyCommands.find((c) => c.recording);
    if (recorder) {
      if (key === 'Escape' || key === '/') {
        // Escape cancels recording; slash is reserved for global search
        set(recorder, 'recording', false);
        set(recorder, 'pattern', recorder.previousPattern);
        recorder = null;
      } else if (key === 'Enter') {
        // Enter finishes recording and removes itself from the buffer
        this.buffer = this.buffer.slice(0, -1);
        this.endRebind(recorder);
        recorder = null;
      } else {
        set(recorder, 'pattern', [...this.buffer]);
      }
    } else {
      if (this.matchedCommands.length) {
        this.matchedCommands.forEach((command) => {
          if (
            this.enabled ||
            command.label === 'Show Keyboard Shortcuts' ||
            command.label === 'Hide Keyboard Shortcuts'
          ) {
            event.preventDefault();
            command.action();
          }
        });
        this.clearBuffer();
      }
    }
    yield timeout(DEBOUNCE_MS);
    if (recorder) {
      this.endRebind(recorder);
    }
    this.clearBuffer();
  }

  get matchedCommands() {
    // Shiftless Buffer: handle the case where use is holding shift (to see shortcut hints) and typing a key command
    const shiftlessBuffer = this.buffer.map((key) =>
      key.replace('Shift+', '').toLowerCase()
    );

    // Shift Friendly Buffer: If you hold Shift and type 0 and 1, it'll output as ['Shift+0', 'Shift+1'].
    // Instead, translate that to ['Shift+01'] for clearer UX
    const shiftFriendlyBuffer = [
      `Shift+${this.buffer.map((key) => key.replace('Shift+', '')).join('')}`,
    ];

    // Ember Compare: returns 0 if there's no diff between arrays.
    const matches = this.keyCommands.filter((command) => {
      return (
        command.action &&
        (!compare(command.pattern, this.buffer) ||
          (command.requireModifier
            ? false
            : !compare(command.pattern, shiftlessBuffer)) ||
          (command.requireModifier
            ? false
            : !compare(command.pattern, shiftFriendlyBuffer)))
      );
    });
    return matches;
  }

  clearBuffer() {
    this.buffer.clear();
  }

  listenForKeypress() {
    set(this, '_keyDownHandler', this.recordKeypress.bind(this, 'press'));
    document.addEventListener('keydown', this._keyDownHandler);
    set(this, '_keyUpHandler', this.recordKeypress.bind(this, 'release'));
    document.addEventListener('keyup', this._keyUpHandler);
  }

  willDestroy() {
    document.removeEventListener('keydown', this._keyDownHandler);
    document.removeEventListener('keyup', this._keyUpHandler);
  }
}
