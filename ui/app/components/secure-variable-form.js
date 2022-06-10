// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
import { trimPath } from '../helpers/trim-path';

export default class SecureVariableFormComponent extends Component {
  @service router;

  @tracked
  shouldHideValues = true;

  /**
   * @typedef {Object} DuplicatePathWarning
   * @property {string} path
   */

  /**
   * @type {DuplicatePathWarning}
   */
  @tracked duplicatePathWarning = null;

  get valueFieldType() {
    return this.shouldHideValues ? 'password' : 'text';
  }

  get shouldDisableSave() {
    return !this.args.model?.path;
  }

  @action
  validatePath(e) {
    const value = trimPath([e.target.value]);
    const existingVariables = this.args.existingVariables || [];
    let existingVariable = existingVariables
      .without(this.args.model)
      .find((v) => v.path === value);
    if (existingVariable) {
      this.duplicatePathWarning = {
        path: existingVariable.path,
      };
    } else {
      this.duplicatePathWarning = null;
    }
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
    this.args.model.setAndTrimPath();
    await this.args.model.save();
    this.router.transitionTo('variables.variable', this.args.model.path);
  }
}
