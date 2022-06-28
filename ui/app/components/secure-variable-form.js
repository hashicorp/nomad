// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
import { trimPath } from '../helpers/trim-path';
import { copy } from 'ember-copy';
import EmberObject from '@ember/object';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';
import { A } from '@ember/array';
import { set } from '@ember/object';

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
    return this.JSONError || !this.args.model?.path;
  }

  /**
   * @type {MutableArray<{key: string, value: string, warnings: EmberObject}>}
   */
  keyValues = A([]);

  /**
   * @type {Object<string, string>}
   */
  JSONItems = {};

  @action
  establishKeyValues() {
    const keyValues = copy(this.args.model?.keyValues || [])?.map((kv) => {
      return {
        key: kv.key,
        value: kv.value,
        warnings: EmberObject.create(),
      };
    });

    /**
     * Appends a row to the end of the Items list if you're editing an existing variable.
     * This will allow it to auto-focus and make all other rows deletable
     */
    // TODO: make the object pushed a const object
    if (!this.args.model?.isNew && this.view === 'table') {
      keyValues.pushObject({
        key: '',
        value: '',
        warnings: EmberObject.create(),
      });
    }
    this.keyValues = keyValues;

    this.JSONItems = this.keyValues.reduce((acc, { key, value }) => {
      acc[key] = value;
      return acc;
    }, {});
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
    if (e.type === 'submit') {
      e.preventDefault();
    }
    // TODO: temp, hacky way to force translation to tabular keyValues
    if (this.view === 'json') {
      this.translateAndValidateItems('table');
    }
    try {
      const nonEmptyItems = A(
        this.keyValues.filter((item) => item.key.trim() && item.value)
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

  //#region JSON Editing

  view = this.args.view;
  // Prevent duplicate onUpdate events when @view is set to its already-existing value,
  // which happens because parent's queryParams and toggle button both resolve independently.
  @action onViewChange([view]) {
    if (view !== this.view) {
      set(this, 'view', view);
      this.translateAndValidateItems(view);
    }
  }

  @action
  translateAndValidateItems(view) {
    // TODO: move the translation functions in serializers/variable.js to generic importable functions.
    if (view === 'json') {
      // Translate table to JSON
      set(
        this,
        'JSONItems',
        this.keyValues
          .filter((item) => item.key.trim() && item.value) // remove empty items when translating to JSON
          .reduce((acc, { key, value }) => {
            acc[key] = value;
            return acc;
          }, {})
      );
    } else if (view === 'table') {
      // Translate JSON to table
      set(
        this,
        'keyValues',
        A(
          Object.entries(this.JSONItems).map(([key, value]) => {
            return {
              key,
              value,
              warnings: EmberObject.create(),
            };
          })
        )
      );
    }

    // Reset any error state, since the errorring json will not persist
    set(this, 'JSONError', null);
  }

  get stringifiedItems() {
    return JSON.stringify(this.args.model.items, null, 2);
  }

  /**
   * @type {string}
   */
  @tracked JSONError = null;
  /**
   *
   * @param {string} value
   */
  @action updateCode(value, codemirror) {
    codemirror.performLint();
    const hasErrors = codemirror?.state.lint.marked?.length > 0;
    if (hasErrors) {
      set(this, 'JSONError', 'Invalid JSON');
    } else {
      set(this, 'JSONError', null);
      set(this, 'JSONItems', JSON.parse(value));
    }
  }
  //#endregion JSON Editing
}
