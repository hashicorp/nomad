import { inject as service } from '@ember/service';
import Modifier from 'ember-modifier';
import { registerDestructor } from '@ember/destroyable';

export default class KeyboardShortcutModifier extends Modifier {
  @service keyboard;

  modify(
    element,
    [eventName],
    {
      label,
      pattern = '',
      action = () => {},
      menuLevel = false,
      enumerated = false,
    }
  ) {
    let commands = [
      {
        label,
        action,
        pattern,
        element,
        menuLevel,
        enumerated,
      },
    ];
    this.keyboard.addCommands(commands);
    registerDestructor(this, () => {
      console.log('regDest', commands);
      this.keyboard.removeCommands(commands);
    });
  }
}
