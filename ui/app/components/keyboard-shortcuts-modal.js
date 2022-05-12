import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { htmlSafe } from '@ember/template';
import { computed } from '@ember/object';
import Tether from 'tether';

export default class KeyboardShortcutsModalComponent extends Component {
  @service keyboard;

  escapeCommand = {
    label: 'Hide Keyboard Shortcuts',
    pattern: ['Escape'],
    action: () => {
      this.keyboard.shortcutsVisible = false;
    },
  };

  /**
   * commands: filter keyCommands to those that have an action and a label,
   * to distinguish between those that are just visual hints of existing commands
   */
  @computed('keyboard.keyCommands.length')
  get commands() {
    return this.keyboard.keyCommands.filter((c) => c.label && c.action);
  }

  /**
   * hints: filter keyCommands to those that have an element property,
   * and then compute a position on screen to place the hint.
   */
  @computed('keyboard.keyCommands.length', 'keyboard.displayHints')
  get hints() {
    console.log('HINTS RECOMPUTE');
    return this.keyboard.keyCommands.filter((c) => c.element);
    // .map((c) => {
    //   c.style = htmlSafe(
    //     `transform: translate(${c.element.getBoundingClientRect().left}px, ${
    //       c.element.getBoundingClientRect().top
    //     }px);`
    //   );
    //   return c;
    // });
  }

  tetherToElement(self, _, { element }) {
    let binder = new Tether({
      element: self,
      target: element,
      attachment: 'top left',
      targetAttachment: 'top left',
      targetModifier: 'visible',
    });
  }
  untetherFromElement(self, _, { element }) {
    // TODO: call binder.destroy, probably componentize hint.
    let binder = new Tether({
      element: self,
      target: element,
      attachment: 'top left',
      targetAttachment: 'top left',
      targetModifier: 'visible',
    });
  }
}
