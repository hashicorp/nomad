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
import { action } from '@ember/object';
import { guidFor } from '@ember/object/internals';
import { assert } from '@ember/debug';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';

const DEBOUNCE_MS = 750;

// Shit modifies event.key to a symbol; get the digit equivalent to perform commands
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

export default class KeyboardService extends Service {
  /**
   * @type {EmberRouter}
   */
  @service router;

  @service config;

  @tracked shortcutsVisible = false;
  @tracked buffer = A([]);
  @tracked displayHints = false;

  /**
   * @type {MutableArray<Object>}
   */
  keyCommands = A([
    {
      label: 'Go to Jobs',
      pattern: ['g', 'j'],
      action: () => this.router.transitionTo('jobs'),
    },
    {
      label: 'Go to Storage',
      pattern: ['g', 'r'],
      action: () => this.router.transitionTo('csi.volumes'),
    },
    {
      label: 'Go to Variables',
      pattern: ['g', 'v'],
      action: () => this.router.transitionTo('variables'),
    },
    {
      label: 'Go to Servers',
      pattern: ['g', 's'],
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
    // {
    //   label: 'Previous Subnav',
    //   pattern: ['k'],
    //   action: () => {
    //     this.traverseLinkList(this.subnavLinks, -1);
    //   },
    // },
    // {
    //   label: 'Next Subnav',
    //   pattern: ['j'],
    //   action: () => {
    //     this.traverseLinkList(this.subnavLinks, 1);
    //   },
    // },
    {
      label: 'Next Subnav',
      pattern: ['Shift+ArrowRight'],
      action: () => {
        this.traverseLinkList(this.subnavLinks, 1);
      },
      requireModifier: true,
    },
    {
      label: 'Previous Subnav',
      pattern: ['Shift+ArrowLeft'],
      action: () => {
        this.traverseLinkList(this.subnavLinks, -1);
      },
      requireModifier: true,
    },
    {
      label: 'Previous Main Section',
      pattern: ['Shift+ArrowUp'],
      action: () => {
        this.traverseLinkList(this.navLinks, -1);
      },
      requireModifier: true,
    },
    {
      label: 'Next Main Section',
      pattern: ['Shift+ArrowDown'],
      action: () => {
        this.traverseLinkList(this.navLinks, 1);
      },
      requireModifier: true,
    },
    {
      label: 'Show Keyboard Shortcuts',
      pattern: ['Shift+?'],
      action: () => {
        this.shortcutsVisible = true;
      },
    },
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
      action: () => {
        console.log('Extra Lives +30');
      },
    },
  ]);

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
    const inputElements = ['input', 'textarea'];
    const targetElementName = event.target.nodeName.toLowerCase();
    // Don't fire keypress events from within an input field
    if (!inputElements.includes(targetElementName)) {
      // Treat Shift like a special modifier key.
      // If it's depressed, display shortcuts
      const { key } = event;
      const shifted = event.getModifierState('Shift');
      if (type === 'press') {
        if (key !== 'Shift') {
          this.addKeyToBuffer.perform(key, shifted);
        } else {
          this.displayHints = true;
        }
      } else if (type === 'release') {
        if (key === 'Shift') {
          this.displayHints = false;
        }
      }
    }
  }

  /**
   *
   * @param {KeyboardEvent} key
   * @param {boolean} shifted
   */
  @restartableTask *addKeyToBuffer(key, shifted) {
    // Replace key with its unshifted equivalent if it's a number key
    if (shifted && key in DIGIT_MAP) {
      key = DIGIT_MAP[key];
    }
    this.buffer.pushObject(shifted ? `Shift+${key}` : key);
    if (this.matchedCommands.length) {
      this.matchedCommands.forEach((command) => command.action());

      // TODO: Temporary dev log
      if (this.config.isDev) {
        this.matchedCommands.forEach((command) =>
          console.log('command run', command, command.action.toString())
        );
      }
      this.clearBuffer();
    }
    yield timeout(DEBOUNCE_MS);
    this.clearBuffer();
  }

  get matchedCommands() {
    // Ember Compare: returns 0 if there's no diff between arrays.
    // TODO: do we think this is faster than a pure JS .join("") comparison?

    // Shiftless Buffer: handle the case where use is holding shift (to see shortcut hints) and typing a key command
    const shiftlessBuffer = this.buffer.map((key) =>
      key.replace('Shift+', '').toLowerCase()
    );

    // Shift Friendly Buffer: If you hold Shift and type 0 and 1, it'll output as ['Shift+0', 'Shift+1'].
    // Instead, translate that to ['Shift+01'] for clearer UX
    const shiftFriendlyBuffer = [
      `Shift+${this.buffer.map((key) => key.replace('Shift+', '')).join('')}`,
    ];

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
    document.addEventListener(
      'keydown',
      this.recordKeypress.bind(this, 'press')
    );
    document.addEventListener(
      'keyup',
      this.recordKeypress.bind(this, 'release')
    );
  }
}
