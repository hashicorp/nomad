import { inject as service } from '@ember/service';
import Modifier from 'ember-modifier';
import { registerDestructor } from '@ember/destroyable';

function cleanup(instance) {
  instance.element?.removeEventListener('click', instance.onClick, true);
}

export default class KeyboardShortcutModifier extends Modifier {
  @service keyboard;

  modify(element, [eventName], { label, pattern, action }) {
    let commands = [
      {
        label,
        action,
        pattern,
        element,
      },
    ];
    console.log('commands', commands, pattern, action);
    this.keyboard.addCommands(commands);
    registerDestructor(this, () => {
      this.keyboard.removeCommands(commands);
    });
  }
}
