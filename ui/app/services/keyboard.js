import Service from '@ember/service';
import { inject as service } from '@ember/service';
import { timeout, restartableTask } from 'ember-concurrency';
import { tracked } from '@glimmer/tracking';
import { compare } from '@ember/utils';
import { A } from '@ember/array';
import EmberRouter from '@ember/routing/router';
import { schedule } from '@ember/runloop';
import { action } from '@ember/object';
import { guidFor } from '@ember/object/internals';

const DEBOUNCE_MS = 750;

export default class KeyboardService extends Service {
  /**
   * @type {EmberRouter}
   */
  @service router;

  @service config;

  @tracked shortcutsVisible = false;
  @tracked buffer = A([]);

  keyCommands = [
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
    },
    {
      label: 'Previous Subnav',
      pattern: ['Shift+ArrowLeft'],
      action: () => {
        this.traverseLinkList(this.subnavLinks, -1);
      },
    },
    {
      label: 'Previous Main Section',
      pattern: ['Shift+ArrowUp'],
      action: () => {
        this.traverseLinkList(this.menuLinks, -1);
      },
    },
    {
      label: 'Next Main Section',
      pattern: ['Shift+ArrowDown'],
      action: () => {
        this.traverseLinkList(this.menuLinks, 1);
      },
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
  ];

  addCommands(commands) {
    this.keyCommands.pushObjects(commands);
  }

  removeCommands(commands) {
    this.keyCommands.removeObjects(commands);
  }

  //#region Nav Traversal

  subnavLinks = [];
  menuLinks = [];

  /**
   * Map over a passed element's links and determine if they're routable
   * If so, return them in a transitionTo-able format
   *
   * @param {HTMLElement} element did-insert'd container div/ul
   */
  @action
  registerSubnav(element) {
    this.subnavLinks = Array.from(
      element.querySelectorAll('a:not(.loading)')
    ).map((link) => {
      return {
        route: this.router.recognize(link.getAttribute('href'))?.name,
        parent: guidFor(element),
      };
    });
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

  @action
  registerMainNav(element) {
    this.menuLinks = Array.from(
      element.querySelectorAll('a:not(.loading)')
    ).map((link) => {
      return {
        route: this.router.recognize(link.getAttribute('href'))?.name,
        parent: guidFor(element),
      };
    });
  }

  // get menuLinks() {
  //   const menu = document.getElementsByClassName('menu')[0];
  //   if (menu) {
  //     return Array.from(menu.querySelectorAll('a')).map((link) => {
  //       return this.router.recognize(link.getAttribute('href'))?.name;
  //     });
  //   } else {
  //     return [];
  //   }
  // }

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
   * @param {KeyboardEvent} event
   */
  recordKeypress(event) {
    const inputElements = ['input', 'textarea'];
    const targetElementName = event.target.nodeName.toLowerCase();
    // Don't fire keypress events from within an input field
    if (!inputElements.includes(targetElementName)) {
      // Treat Shift like a special modifier key. May expand to more later.
      const { key } = event;
      const shifted = event.getModifierState('Shift');
      if (key !== 'Shift') {
        this.addKeyToBuffer.perform(key, shifted);
      }
    }
  }

  /**
   *
   * @param {KeyboardEvent} key
   * @param {boolean} shifted
   */
  @restartableTask *addKeyToBuffer(key, shifted) {
    this.buffer.pushObject(shifted ? `Shift+${key}` : key);
    if (this.matchedCommands.length) {
      this.matchedCommands.forEach((command) => command.action());

      // TODO: Temporary dev log
      if (this.config.isDev) {
        this.matchedCommands.forEach((command) =>
          console.log('command run', command)
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
    const matches = this.keyCommands.filter(
      (command) => !compare(command.pattern, this.buffer)
    );
    return matches;
  }

  clearBuffer() {
    this.buffer.clear();
  }

  listenForKeypress() {
    document.addEventListener('keydown', this.recordKeypress.bind(this));
  }
}
