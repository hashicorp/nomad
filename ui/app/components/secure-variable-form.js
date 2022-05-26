// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { A } from '@ember/array';

export default class SecureVariableFormComponent extends Component {
  path = '';
  keyValues = A([{ key: '', value: '' }]);

  @tracked
  shouldHideValues = true;

  get valueFieldType() {
    return this.shouldHideValues ? 'password' : 'text';
  }

  @action
  toggleShowHide() {
    this.shouldHideValues = !this.shouldHideValues;
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
