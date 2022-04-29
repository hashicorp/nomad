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

  /**
   *
   * @param {Array<string>} links - array of root.branch.twig strings
   * @param {number} traverseBy - positive or negative number to move along links
   */
  traverseSubnav(links, traverseBy) {
    // afterRender because LinkTos evaluate their href value at render time
    schedule('afterRender', () => {
      if (links.length) {
        const activeLink = links.find((link) => this.router.isActive(link));
        if (activeLink) {
          // TODO: test this, maybe write less defensively
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
      action: () => {},
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
    {
      label: 'Next Subnav',
      pattern: ['j'],
      action: () => {
        this.traverseSubnav(this.subnavLinks, 1);
      },
    },
    {
      label: 'Previous Subnav',
      pattern: ['k'],
      action: () => {
        this.traverseSubnav(this.subnavLinks, -1);
      },
    },
  ];

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
    if (this.matchedCommand) {
      this.matchedCommand.action();
      this.clearBuffer();
    }
    yield timeout(DEBOUNCE_MS);
    this.clearBuffer();
  }

  // 👻 TODO, temp, dev.
  @tracked matchedCommandGhost = '';

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
      }, 2000);
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
