// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
import { trimPath } from '../helpers/trim-path';
import { copy } from 'ember-copy';
import EmberObject from '@ember/object';

export default class SecureVariableFormComponent extends Component {
  @service router;
  @service flashMessages;

  /**
   * @typedef {Object} DuplicatePathWarning
   * @property {string} path
   */

  /**
   * @type {DuplicatePathWarning}
   */
  @tracked duplicatePathWarning = null;

  get shouldDisableSave() {
    return !this.args.model?.path;
  }

  @tracked keyValues = copy(this.args.model?.keyValues)?.map((kv) => {
    return {
      key: kv.key,
      value: kv.value,
      warnings: EmberObject.create(),
    };
  });

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
  validateKey(entry, e) {
    const value = e.target.value;
    if (value.includes('.')) {
      entry.warnings.set('dottedKeyError', 'Key should not contain a period.');
    } else {
      delete entry.warnings.dottedKeyError;
      entry.warnings.notifyPropertyChange('dottedKeyError');
    }
  }

  @action appendRow() {
    this.keyValues.pushObject({
      key: '',
      value: '',
      warnings: EmberObject.create(),
    });
  }

  @action deleteRow(row) {
    this.keyValues.removeObject(row);
  }

  @action
  async save(e) {
    e.preventDefault();
    try {
      const nonEmptyItems = this.keyValues.filter(
        (item) => item.key.trim() && item.value
      );
      if (!nonEmptyItems.length) {
        throw new Error('Please provide at least one key/value pair.');
      } else {
        this.keyValues = nonEmptyItems;
      }

      this.args.model.set('keyValues', this.keyValues);
      this.args.model.setAndTrimPath();
      await this.args.model.save();

      this.flashMessages.add({
        title: 'Secure Variable saved',
        message: `${this.args.model.path} successfully saved`,
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
        showProgress: true,
      });
      this.router.transitionTo('variables.variable', this.args.model.path);
    } catch (error) {
      this.flashMessages.add({
        title: `Error saving ${this.args.model.path}`,
        message: error,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });
    }
  }
}
