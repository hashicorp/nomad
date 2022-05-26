// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { A } from '@ember/array';

export default class SecureVariableFormComponent extends Component {
  path = '';
  keyValues = A([{ key: '', value: '' }]);

  @tracked
  hideValues = true;

  get valueFieldType() {
    return this.hideValues ? 'password' : 'text';
  }

  @action
  toggleShowHide() {
    this.hideValues = !this.hideValues;
  }

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
