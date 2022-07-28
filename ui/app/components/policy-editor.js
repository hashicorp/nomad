import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class PolicyEditorComponent extends Component {
  @action onSave() {
    this.args.onSave();
  }

  get shouldShowLinkedEntities() {
    return (
      this.args.policy.nameLinkedEntities?.namespace ||
      this.args.policy.nameLinkedEntities?.job ||
      this.args.policy.nameLinkedEntities?.group ||
      this.args.policy.nameLinkedEntities?.task
    );
  }
}
