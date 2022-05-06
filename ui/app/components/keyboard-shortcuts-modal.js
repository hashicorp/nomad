import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

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
}
