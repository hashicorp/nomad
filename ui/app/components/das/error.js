import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class DasErrorComponent extends Component {
  @action
  dismissClicked() {
    this.args.proceed({ manuallyDismissed: true });
  }
}
