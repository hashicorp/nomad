// @ts-check

import { action } from '@ember/object';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

export default class InputGroup extends Component {
  @tracked isObscured = true;

  get inputType() {
    return this.isObscured ? 'password' : 'text';
  }

  @action
  toggleInputType() {
    this.isObscured = !this.isObscured;
  }
}
