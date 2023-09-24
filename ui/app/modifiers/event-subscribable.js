import Modifier from 'ember-modifier';
import { inject as service } from '@ember/service';
import { registerDestructor } from '@ember/destroyable';

export default class EventSubscribableModifier extends Modifier {
  @service events;
  modify(element, _positional, { label, action = () => {} }) {
    let commands = [
      {
        label,
        action,
      },
    ];

    element.addEventListener('click', action);
    element.classList.add('event-subscribable');

    // this.keyboard.addCommands(commands);
    registerDestructor(this, () => {
      element.removeEventListener('click', action);
      element.classList.remove('event-subscribable');
      // this.keyboard.removeCommands(commands);
    });
  }
}
