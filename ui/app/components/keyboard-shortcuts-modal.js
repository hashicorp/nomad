import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { action } from '@ember/object';
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
  @computed('keyboard.keyCommands.[]')
  get commands() {
    return this.keyboard.keyCommands.filter((c) => c.label && c.action);
  }

  /**
   * hints: filter keyCommands to those that have an element property,
   * and then compute a position on screen to place the hint.
   */
  @computed('keyboard.{keyCommands.length,displayHints}')
  get hints() {
    if (this.keyboard.displayHints) {
      return this.keyboard.keyCommands.filter((c) => c.element);
    } else {
      return [];
    }
  }

  tetherToElement(self, _, { element, hint }) {
    let binder = new Tether({
      element: self,
      target: element,
      attachment: 'top left',
      targetAttachment: 'top left',
      targetModifier: 'visible',
    });
    hint.binder = binder;
  }
  untetherFromElement(self, _, { hint }) {
    hint.binder.destroy();
  }

  @action toggleListener() {
    this.keyboard.enabled = !this.keyboard.enabled;
  }
}
