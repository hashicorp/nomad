import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class SecureVariableFormComponent extends Component {
  key = 'a';
  value = 'b';
  path = 'c';

  @action
  saveNewVariable(e) {
    e.preventDefault();
    const { path, key, value } = this;
    this.args.save({ path, key, value });
  }
}
