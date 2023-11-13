import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';

export default class ActionsFlyoutComponent extends Component {
  @service nomadActions;

  @alias('nomadActions.flyoutActive') isOpen;
}
