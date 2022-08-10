// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
import { trimPath } from '../helpers/trim-path';
import { copy } from 'ember-copy';
import EmberObject, { set } from '@ember/object';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';
import { A } from '@ember/array';
import { stringifyObject } from 'nomad-ui/helpers/stringify-object';

const EMPTY_KV = {
  key: '',
  value: '',
  warnings: EmberObject.create(),
};

export default class SecureVariableFormComponent extends Component {
  @service flashMessages;
  @service router;
  @service store;

  /**
   * @typedef {Object} DuplicatePathWarning
   * @property {string} path
   */

  /**
   * @type {DuplicatePathWarning}
   */
  @tracked duplicatePathWarning = null;
  @tracked variableNamespace = null;
  @tracked namespaceOptions = null;

  @action
  setNamespace(namespace) {
    this.variableNamespace = namespace;
  }

  @action
  setNamespaceOptions(options) {
    this.namespaceOptions = options;

    // Set first namespace option
    if (options.length) {
      this.variableNamespace = this.args.model.namespace;
    }
  }

  get shouldDisableSave() {
    const disallowedPath =
      this.args.model?.path?.startsWith('nomad/') &&
      !this.args.model?.path?.startsWith('nomad/jobs');
    return !!this.JSONError || !this.args.model?.path || disallowedPath;
  }

  /**
   * @type {MutableArray<{key: string, value: string, warnings: EmberObject}>}
   */
  keyValues = A([]);

  /**
   * @type {string}
   */
  JSONItems = '{}';

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
    if (!this.args.model?.isNew) {
      keyValues.pushObject(copy(EMPTY_KV));
    }
    this.keyValues = keyValues;

    this.JSONItems = stringifyObject([
      this.keyValues.reduce((acc, { key, value }) => {
        acc[key] = value;
        return acc;
      }, {}),
    ]);
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
    // No dots in key names
    if (value.includes('.')) {
      entry.warnings.set('dottedKeyError', 'Key should not contain a period.');
    } else {
      delete entry.warnings.dottedKeyError;
      entry.warnings.notifyPropertyChange('dottedKeyError');
    }

    // no duplicate keys
    const existingKeys = this.keyValues.map((kv) => kv.key);
    if (existingKeys.includes(value)) {
      entry.warnings.set('duplicateKeyError', 'Key already exists.');
    } else {
      delete entry.warnings.duplicateKeyError;
      entry.warnings.notifyPropertyChange('duplicateKeyError');
    }
  }

  @action appendRow() {
    this.keyValues.pushObject(copy(EMPTY_KV));
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

      if (this.args.model?.isNew) {
        if (this.namespaceOptions) {
          this.args.model.set('namespace', this.variableNamespace);
        } else {
          const [namespace] = this.store.peekAll('namespace').toArray();
          this.args.model.set('namespace', namespace.id);
        }
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
        stringifyObject([
          this.keyValues
            .filter((item) => item.key.trim() && item.value) // remove empty items when translating to JSON
            .reduce((acc, { key, value }) => {
              acc[key] = value;
              return acc;
            }, {}),
        ])
      );

      // Give the user a foothold if they're transitioning an empty K/V form into JSON
      if (!Object.keys(JSON.parse(this.JSONItems)).length) {
        set(this, 'JSONItems', stringifyObject([{ '': '' }]));
      }
    } else if (view === 'table') {
      // Translate JSON to table
      set(
        this,
        'keyValues',
        A(
          Object.entries(JSON.parse(this.JSONItems)).map(([key, value]) => {
            return {
              key,
              value: typeof value === 'string' ? value : JSON.stringify(value),
              warnings: EmberObject.create(),
            };
          })
        )
      );

      // If the JSON object is empty at switch time, add an empty KV in to give the user a foothold
      if (!Object.keys(JSON.parse(this.JSONItems)).length) {
        this.appendRow();
      }
    }

    // Reset any error state, since the errorring json will not persist
    set(this, 'JSONError', null);
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
    try {
      const hasLintErrors = codemirror?.state.lint.marked?.length > 0;
      if (hasLintErrors || !JSON.parse(value)) {
        throw new Error('Invalid JSON');
      }

      // "myString" is valid JSON, but it's not a valid Secure Variable.
      // Ditto for an array of objects. We expect a single object to be a Secure Variable.
      const hasFormatErrors =
        JSON.parse(value) instanceof Array ||
        typeof JSON.parse(value) !== 'object';
      if (hasFormatErrors) {
        throw new Error(
          'A Secure Variable must be formatted as a single JSON object'
        );
      }

      set(this, 'JSONError', null);
      set(this, 'JSONItems', value);
    } catch (error) {
      set(this, 'JSONError', error);
    }
  }
  //#endregion JSON Editing

  get shouldShowLinkedEntities() {
    return (
      this.args.model.pathLinkedEntities?.job ||
      this.args.model.pathLinkedEntities?.group ||
      this.args.model.pathLinkedEntities?.task
    );
  }
}
