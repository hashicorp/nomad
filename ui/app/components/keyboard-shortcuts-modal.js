import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { htmlSafe } from '@ember/template';
import { computed } from '@ember/object';

export default class KeyboardShortcutsModalComponent extends Component {
  @service keyboard;

  keyCommands = [
    {
      label: 'Hide Keyboard Shortcuts',
      pattern: ['Escape'],
      action: () => {
        this.keyboard.shortcutsVisible = false;
      },
    },
  ];

  @computed('keyboard.keyCommands', 'keyboard.displayHints')
  get hints() {
    console.log('recomputing hints', this.keyboard.keyCommands.length);
    return this.keyboard.keyCommands
      .filter((c) => c.element)
      .map((c) => {
        c.style = htmlSafe(
          `transform: translate(${c.element.getBoundingClientRect().left}px, ${
            c.element.getBoundingClientRect().top
          }px);`
        );
        return c;
      });
  }
}
