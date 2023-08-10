/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Component from '@glimmer/component';
import { action, computed } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
import { trimPath } from '../helpers/trim-path';
import { copy } from 'ember-copy';
import EmberObject, { set } from '@ember/object';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';
import { A } from '@ember/array';
import { stringifyObject } from 'nomad-ui/helpers/stringify-object';
import notifyConflict from 'nomad-ui/utils/notify-conflict';
import isEqual from 'lodash.isequal';

const EMPTY_KV = {
  key: '',
  value: '',
  warnings: EmberObject.create(),
};

// Capture characters that are not _, letters, or numbers using Unicode.
const invalidKeyCharactersRegex = new RegExp(/[^_\p{Letter}\p{Number}]/gu);

export default class VariableFormComponent extends Component {
  @service notifications;
  @service router;
  @service store;
  @service can;

  @tracked variableNamespace = null;
  @tracked namespaceOptions = null;
  @tracked hasConflict = false;

  /**
   * @typedef {Object} conflictingVariable
   * @property {string} ModifyTime
   * @property {Object} Items
   */

  /**
   * @type {conflictingVariable}
   */
  @tracked conflictingVariable = null;

  @tracked path = '';
  constructor() {
    super(...arguments);
    set(this, 'path', this.args.model.path);
    this.addExitHandler();
  }

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
      this.path?.startsWith('nomad/') &&
      !(
        this.path?.startsWith('nomad/jobs') ||
        (this.path?.startsWith('nomad/job-templates') &&
          trimPath([this.path]) !== 'nomad/job-templates')
      );
    return !!this.JSONError || !this.path || disallowedPath;
  }

  get isJobTemplateVariable() {
    return this.path?.startsWith('nomad/job-templates/');
  }

  get jobTemplateName() {
    return this.path.split('nomad/job-templates/').slice(-1);
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
    set(this, 'keyValues', keyValues);

    this.JSONItems = stringifyObject([
      this.keyValues.reduce((acc, { key, value }) => {
        acc[key] = value;
        return acc;
      }, {}),
    ]);
  }

  /**
   * @typedef {Object} DuplicatePathWarning
   * @property {string} path
   */

  /**
   * @type {DuplicatePathWarning}
   */
  get duplicatePathWarning() {
    const existingVariables = this.args.existingVariables || [];
    const pathValue = trimPath([this.path]);
    let existingVariable = existingVariables
      .without(this.args.model)
      .find(
        (v) =>
          v.path === pathValue &&
          (v.namespace === this.variableNamespace || !this.variableNamespace)
      );
    if (existingVariable) {
      return {
        path: existingVariable.path,
      };
    } else {
      return null;
    }
  }

  @action
  validateKey(entry, e) {
    const value = e.target.value;
    // Only letters, numbers, and _ are allowed in keys
    const invalidChars = value.match(invalidKeyCharactersRegex);
    if (invalidChars) {
      const invalidCharsOuput = [...new Set(invalidChars)]
        .sort()
        .map((c) => `'${c}'`);
      entry.warnings.set(
        'dottedKeyError',
        `${value} contains characters [${invalidCharsOuput}] that require the "index" function for direct access in templates.`
      );
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
    // Clear our any entity errors
    let newRow = copy(EMPTY_KV);
    newRow.warnings = EmberObject.create();
    this.keyValues.pushObject(newRow);
  }

  @action deleteRow(row) {
    this.keyValues.removeObject(row);
  }

  @action refresh() {
    window.location.reload();
  }

  @action saveWithOverwrite(e) {
    set(this, 'conflictingVariable', null);
    this.save(e, true);
  }

  /**
   *
   * @param {KeyboardEvent} e
   */
  @action setModelPath(e) {
    set(this.args.model, 'path', e.target.value);
  }

  @action updateKeyValue(key, value) {
    if (this.keyValues.find((kv) => kv.key === key)) {
      this.keyValues.find((kv) => kv.key === key).value = value;
    } else {
      this.keyValues.pushObject({ key, value, warnings: EmberObject.create() });
    }
  }

  @action
  async save(e, overwrite = false) {
    if (e.type === 'submit') {
      e.preventDefault();
    }

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
        set(this, 'keyValues', nonEmptyItems);
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
      this.args.model.set('path', this.path);
      this.args.model.setAndTrimPath();
      await this.args.model.save({ adapterOptions: { overwrite } });

      this.notifications.add({
        title: 'Variable saved',
        message: `${this.path} successfully saved`,
        color: 'success',
      });

      if (
        this.can.can('read job', null, {
          namespace: this.variableNamespace || 'default',
        })
      ) {
        this.updateJobVariables(this.args.model.pathLinkedEntities.job);
      }

      this.removeExitHandler();
      this.router.transitionTo('variables.variable', this.args.model.id);
    } catch (error) {
      notifyConflict(this)(error);
      if (!this.hasConflict) {
        this.notifications.add({
          title: `Error saving ${this.path}`,
          message: error,
          color: 'critical',
          sticky: true,
        });
      } else {
        if (error.errors[0]?.detail) {
          set(this, 'conflictingVariable', error.errors[0].detail);
        }
        window.scrollTo(0, 0); // because the k/v list may be long, ensure the user is snapped to top to read error
      }
    }
  }

  /**
   * A job, its task groups, and tasks, all have a getter called pathLinkedVariable.
   * These are dependent on a variables list that may already be established. If a variable
   * is added or removed, this function will update job.variables[] list to reflect the change.
   * and force an update to the job's pathLinkedVariable getter.
   */
  async updateJobVariables(jobName) {
    if (!jobName) {
      return;
    }
    const fullJobId = JSON.stringify([
      jobName,
      this.variableNamespace || 'default',
    ]);
    let job = await this.store.findRecord('job', fullJobId, { reload: true });
    if (job) {
      job.variables.pushObject(this.args.model);
    }
  }

  //#region JSON Editing

  view = this.args.view;

  get isJSONView() {
    return this.args.view === 'json';
  }

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

      // "myString" is valid JSON, but it's not a valid Variable.
      // Ditto for an array of objects. We expect a single object to be a Variable.
      const hasFormatErrors =
        JSON.parse(value) instanceof Array ||
        typeof JSON.parse(value) !== 'object';
      if (hasFormatErrors) {
        throw new Error('A Variable must be formatted as a single JSON object');
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
      this.args.model.pathLinkedEntities?.task ||
      trimPath([this.path]) === 'nomad/jobs'
    );
  }

  //#region Unsaved Changes Confirmation

  hasRemovedExitHandler = false;

  @computed(
    'args.model.{keyValues,path}',
    'keyValues.@each.{key,value}',
    'path'
  )
  get hasUserModifiedAttributes() {
    const compactedBasicKVs = this.keyValues
      .map((kv) => ({ key: kv.key, value: kv.value }))
      .filter((kv) => kv.key || kv.value);
    const compactedPassedKVs = this.args.model.keyValues.filter(
      (kv) => kv.key || kv.value
    );
    const unequal =
      !isEqual(compactedBasicKVs, compactedPassedKVs) ||
      !isEqual(this.path, this.args.model.path);
    return unequal;
  }

  addExitHandler() {
    this.router.on('routeWillChange', this, this.confirmExit);
  }

  removeExitHandler() {
    if (!this.hasRemovedExitHandler) {
      this.router.off('routeWillChange', this, this.confirmExit);
      this.hasRemovedExitHandler = true;
    }
  }

  confirmExit(transition) {
    if (transition.isAborted || transition.queryParamsOnly) return;

    if (this.hasUserModifiedAttributes) {
      if (
        !confirm(
          'Your variable has unsaved changes. Are you sure you want to leave?'
        )
      ) {
        transition.abort();
      } else {
        this.removeExitHandler();
      }
    }
  }

  willDestroy() {
    super.willDestroy(...arguments);
    this.removeExitHandler();
  }

  //#endregion Unsaved Changes Confirmation
}
