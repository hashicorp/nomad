import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
import { trimPath } from '../helpers/trim-path';
export default class SecureVariableFormComponent extends Component {
  @service router;

  @tracked
  shouldHideValues = true;

  get valueFieldType() {
    return this.shouldHideValues ? 'password' : 'text';
  }

  get shouldDisableSave() {
    return !this.args.model?.path;
  }

  @action
  toggleShowHide() {
    this.shouldHideValues = !this.shouldHideValues;
  }

  @action appendRow() {
    this.args.model.keyValues.pushObject({
      key: '',
      value: '',
    });
  }

  @action deleteRow(row) {
    this.args.model.keyValues.removeObject(row);
  }

  @action
  async save(e) {
    e.preventDefault();
    this.args.model.path = trimPath([this.args.model.path]); // remove starting and trailing slashes
    this.args.model.id = this.args.model.path;

    const transitionTarget = this.args.model.isNew
      ? 'variables'
      : 'variables.variable';

    await this.args.model.save();
    this.router.transitionTo(transitionTarget);
  }
}
