import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class PolicyEditorComponent extends Component {
  @action onSave() {
    this.args.onSave();
  }
}
