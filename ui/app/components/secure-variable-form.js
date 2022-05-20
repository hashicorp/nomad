import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class SecureVariableFormComponent extends Component {
  keyValues = [{ key: '', value: '' }];

  @action appendRow() {
    this.keyValues.pushObject({
      key: '',
      value: '',
    });
  }

  @action
  saveNewVariable(e) {
    e.preventDefault();
    const { path, keyValues } = this;
    this.args.save({ path, keyValues });
  }
}
