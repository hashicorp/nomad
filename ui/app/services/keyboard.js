import Service from '@ember/service';
import { inject as service } from '@ember/service';
import { timeout, restartableTask } from 'ember-concurrency';
import { tracked } from '@glimmer/tracking';
import { compare } from '@ember/utils';
import { A } from '@ember/array';
import EmberRouter from '@ember/routing/router';
import { schedule } from '@ember/runloop';
import { run } from '@ember/runloop';

const DEBOUNCE_MS = 750;

export default class KeyboardService extends Service {
  /**
   * @type {EmberRouter}
   */
  @service router;
  @tracked shortcutsVisible = false;
  @tracked buffer = A([]);
  @tracked matchedCommandGhost = ''; // 👻 TODO, temp, dev.

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
    {
      label: 'Next Subnav',
      pattern: ['j'],
      action: () => {
        this.traverseLinkList(this.subnavLinks, 1);
      },
    },
    {
      label: 'Next Subnav',
      pattern: ['Shift+ArrowRight'],
      action: () => {
        this.traverseLinkList(this.subnavLinks, 1);
      },
    },
    {
      label: 'Previous Subnav',
      pattern: ['k'],
      action: () => {
        this.traverseLinkList(this.subnavLinks, -1);
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
      label: 'Hide Keyboard Shortcuts',
      pattern: ['Escape'],
      action: () => {
        this.shortcutsVisible = false;
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

  //#region Nav Traversal

  // 1. see if there's an .is-subnav element on the page
  // 2. if so, map over its links and use router.recognize to extract route patterns
  // (changes "/ui/jobs/jbod-firewall-2@namespace-2/definition" into "jobs.job.definition")
  get subnavLinks() {
    // TODO: this feels very non-Embery. Gotta see if there's a better way to handle this.
    const subnav = document.getElementsByClassName('is-subnav')[0];
    if (subnav) {
      return Array.from(subnav.querySelectorAll('a')).map((link) => {
        return this.router.recognize(link.getAttribute('href'))?.name;
      });
    } else {
      return [];
    }
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
        let activeLink = links.find((link) => this.router.isActive(link));

        // If no activeLink, means we're nested within a primary section.
        // Luckily, Ember's RouteInfo.find() gives us access to parents and connected leaves of a route.
        // So, if we're on /csi/volumes but the nav link is to /csi, we'll .find() it.
        // Similarly, /job/:job/taskgroupid/index will find /job.
        if (!activeLink) {
          activeLink = links.find((link) => {
            return this.router.currentRoute.find((r) => {
              return r.name === link || `${r.name}.index` === link;
            });
          });
        }

        if (activeLink) {
          const activeLinkPosition = links.indexOf(activeLink);
          const nextPosition = activeLinkPosition + traverseBy;

          // Modulo (%) logic: if the next position is longer than the array, wrap to 0.
          // If it's before the beginning, wrap to the end.
          const nextLink =
            links[
              ((nextPosition % links.length) + links.length) % links.length
            ];

          this.router.transitionTo(nextLink);
        }
      }
    });
  }

  get menuLinks() {
    const menu = document.getElementsByClassName('menu')[0];
    if (menu) {
      return Array.from(menu.querySelectorAll('a')).map((link) => {
        return this.router.recognize(link.getAttribute('href'))?.name;
      });
    } else {
      return [];
    }
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
    if (this.matchedCommand) {
      this.matchedCommand.action();
      yield timeout(DEBOUNCE_MS / 2);
      this.clearBuffer();
    }
    yield timeout(DEBOUNCE_MS);
    this.clearBuffer();
  }

  get matchedCommand() {
    // Ember Compare: returns 0 if there's no diff between arrays.
    // TODO: do we think this is faster than a pure JS .join("") comparison?
    const match = this.keyCommands.find(
      (command) => !compare(command.pattern, this.buffer)
    );

    // 👻 TODO, temp, dev.
    if (match) {
      this.matchedCommandGhost = match?.label;
      run.later(() => {
        this.matchedCommandGhost = '';
      }, DEBOUNCE_MS * 2);
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
