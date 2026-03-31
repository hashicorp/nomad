/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { concat, fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { trackedArray } from '@ember/reactive/collections';
import { service } from '@ember/service';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import didUpdate from '@ember/render-modifiers/modifiers/did-update';
import { copy } from 'ember-copy';
import EmberObject from '@ember/object';
import { and, eq, not, or } from 'ember-truth-helpers';
import {
  HdsButton,
  HdsCopySnippet,
  HdsFormTextInputField,
} from '@hashicorp/design-system-components/components';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import autofocus from 'nomad-ui/modifiers/autofocus';
import codeMirror from 'nomad-ui/modifiers/code-mirror';
import { stringifyObject as stringifyObjectValue } from 'nomad-ui/helpers/stringify-object';
import stringifyObject from 'nomad-ui/helpers/stringify-object';
import formatTs from 'nomad-ui/helpers/format-ts';
import { trimPath } from 'nomad-ui/helpers/trim-path';
import VariableFormInputGroup from 'nomad-ui/components/variable-form/input-group';
import VariableFormJobTemplateEditor from 'nomad-ui/components/variable-form/job-template-editor';
import VariableFormNamespaceFilter from 'nomad-ui/components/variable-form/namespace-filter';
import VariableFormRelatedEntities from 'nomad-ui/components/variable-form/related-entities';
import notifyConflict from 'nomad-ui/utils/notify-conflict';
import pluralize from 'nomad-ui/helpers/pluralize';
import isEqual from 'fast-deep-equal';

const EMPTY_KV = {
  key: '',
  value: '',
  warnings: EmberObject.create(),
};

const invalidKeyCharactersRegex = new RegExp(/[^_\p{Letter}\p{Number}]/gu);

export default class VariableForm extends Component {
  @service notifications;
  @service router;
  @service store;
  @service abilities;

  @tracked variableNamespace = null;
  @tracked namespaceOptions = null;
  @tracked hasConflict = false;
  @tracked conflictingVariable = null;
  @tracked path = '';
  @tracked JSONError = null;
  @tracked keyValues = trackedArray([]);
  @tracked view = 'table';
  @tracked JSONItems = '{}';

  hasRemovedExitHandler = false;

  constructor() {
    super(...arguments);
    this.path = this.args.model.path;
    this.view = this.args.view;
    this.addExitHandler();
  }

  setNamespace = (namespace) => {
    this.variableNamespace = namespace;
  };

  setNamespaceOptions = (options) => {
    this.namespaceOptions = options;

    if (options.length) {
      this.variableNamespace = this.args.model.namespace;
    }
  };

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

  establishKeyValues = () => {
    const keyValues = (copy(this.args.model?.keyValues || []) || []).map(
      (kv) => ({
        key: kv.key,
        value: kv.value,
        warnings: EmberObject.create(),
      }),
    );

    if (!this.args.model?.isNew) {
      keyValues.push(copy(EMPTY_KV));
    }
    this.keyValues = trackedArray(keyValues);

    this.JSONItems = stringifyObjectValue([
      this.keyValues.reduce((accumulator, { key, value }) => {
        accumulator[key] = value;
        return accumulator;
      }, {}),
    ]);
  };

  get duplicatePathWarning() {
    const existingVariables = normalizeCollection(this.args.existingVariables);
    const pathValue = trimPath([this.path]);
    const existingVariable = existingVariables
      .filter((variable) => variable !== this.args.model)
      .find(
        (variable) =>
          variable.path === pathValue &&
          (variable.namespace === this.variableNamespace ||
            !this.variableNamespace),
      );

    if (existingVariable) {
      return {
        path: existingVariable.path,
      };
    }

    return null;
  }

  get hasInvalidPath() {
    return !new RegExp('^[a-zA-Z0-9-_~/]{1,128}$').test(trimPath([this.path]));
  }

  validateKey = (entry, event) => {
    const value = event.target.value;
    const invalidChars = value.match(invalidKeyCharactersRegex);
    if (invalidChars) {
      const invalidCharsOutput = [...new Set(invalidChars)]
        .sort()
        .map((character) => `'${character}'`);
      entry.warnings.set(
        'dottedKeyError',
        `${value} contains characters [${invalidCharsOutput}] that require the "index" function for direct access in templates.`,
      );
    } else {
      delete entry.warnings.dottedKeyError;
      entry.warnings.notifyPropertyChange('dottedKeyError');
    }

    const existingKeys = this.keyValues.map((kv) => kv.key);
    if (existingKeys.includes(value)) {
      entry.warnings.set('duplicateKeyError', 'Key already exists.');
    } else {
      delete entry.warnings.duplicateKeyError;
      entry.warnings.notifyPropertyChange('duplicateKeyError');
    }

    entry.key = value;
    this.keyValues = trackedArray([...this.keyValues]);
  };

  setEntryValue = (entry, valueOrEvent) => {
    const nextValue =
      typeof valueOrEvent === 'string'
        ? valueOrEvent
        : (valueOrEvent?.target?.value ??
          valueOrEvent?.detail?.value ??
          (typeof valueOrEvent?.detail === 'string'
            ? valueOrEvent.detail
            : ''));

    entry.value = nextValue;
    this.keyValues = trackedArray([...this.keyValues]);
  };

  appendRow = () => {
    const newRow = copy(EMPTY_KV);
    newRow.warnings = EmberObject.create();
    this.keyValues.push(newRow);
  };

  deleteRow = (row) => {
    const index = this.keyValues.indexOf(row);
    if (index > -1) {
      this.keyValues.splice(index, 1);
    }
  };

  refresh = () => {
    window.location.reload();
  };

  saveWithOverwrite = (event) => {
    this.conflictingVariable = null;
    this.save(event, true);
  };

  setModelProperty = (key, value) => {
    const model = this.args.model;

    if (typeof model?.set === 'function') {
      model.set(key, value);
      return;
    }

    if (model) {
      model[key] = value;
    }
  };

  setModelPath = (event) => {
    this.path = event.target.value;
    this.setModelProperty('path', event.target.value);
  };

  updateKeyValue = (key, value) => {
    const existing = this.keyValues.find((kv) => kv.key === key);
    if (existing) {
      existing.value = value;
    } else {
      this.keyValues.push({ key, value, warnings: EmberObject.create() });
    }
  };

  save = async (event, overwrite = false) => {
    event?.preventDefault?.();

    if (this.view === 'json') {
      this.translateAndValidateItems('table');
    }
    try {
      const nonEmptyItems = this.keyValues.filter(
        (item) => item.key.trim() && item.value,
      );
      if (!nonEmptyItems.length) {
        throw new Error('Please provide at least one key/value pair.');
      } else {
        this.keyValues = trackedArray(nonEmptyItems);
      }

      if (this.args.model?.isNew) {
        if (this.namespaceOptions) {
          this.setModelProperty('namespace', this.variableNamespace);
        } else {
          const [namespace] = [...this.store.peekAll('namespace')];
          this.setModelProperty('namespace', namespace.id);
        }
      }

      this.setModelProperty('keyValues', this.keyValues);
      this.setModelProperty('path', this.path);

      if (typeof this.args.model?.setAndTrimPath === 'function') {
        this.args.model.setAndTrimPath();
      } else {
        this.setModelProperty('path', trimPath([this.path]));
      }

      await this.args.model.save({ adapterOptions: { overwrite } });

      this.notifications.add({
        title: 'Variable saved',
        message: `${this.path} successfully saved`,
        color: 'success',
      });

      if (
        this.abilities.can('read job', null, {
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
        let errorMessage = error;
        if (error.errors && error.errors.length > 0) {
          const nameInvalidError = error.errors.find(
            (err) => err.status === 400,
          );
          if (nameInvalidError) {
            errorMessage = nameInvalidError.detail;
          }
        }

        this.notifications.add({
          title: `Error saving ${this.path}`,
          message: errorMessage,
          color: 'critical',
          sticky: true,
        });
      } else {
        if (error.errors[0]?.detail) {
          this.conflictingVariable = error.errors[0].detail;
        }
        window.scrollTo(0, 0);
      }
    }
  };

  async updateJobVariables(jobName) {
    if (!jobName) {
      return;
    }
    const fullJobId = JSON.stringify([
      jobName,
      this.variableNamespace || 'default',
    ]);
    const job = await this.store.findRecord('job', fullJobId, { reload: true });
    if (job) {
      if (typeof job.variables?.pushObject === 'function') {
        job.variables.pushObject(this.args.model);
      } else {
        const variables = normalizeCollection(job.variables);
        job.variables = [...variables, this.args.model];
      }
    }
  }

  get lastKeyValue() {
    return this.keyValues[this.keyValues.length - 1];
  }

  get isJSONView() {
    return this.args.view === 'json';
  }

  onViewChange = () => {
    if (this.args.view !== this.view) {
      this.translateAndValidateItems(this.args.view);
      this.view = this.args.view;
    }
  };

  translateAndValidateItems = (view) => {
    if (view === 'json') {
      const items = this.liveTableItemsForJSON();

      this.JSONItems = stringifyObjectValue([
        items
          .filter((item) => item.key.trim() && item.value)
          .reduce((accumulator, { key, value }) => {
            const normalizedKey = key.trim();
            accumulator[normalizedKey] = value;
            return accumulator;
          }, {}),
      ]);

      if (!Object.keys(JSON.parse(this.JSONItems)).length) {
        this.JSONItems = stringifyObjectValue([{ '': '' }]);
      }
    } else if (view === 'table') {
      this.keyValues = trackedArray(
        Object.entries(JSON.parse(this.JSONItems)).map(([key, value]) => ({
          key,
          value: typeof value === 'string' ? value : JSON.stringify(value),
          warnings: EmberObject.create(),
        })),
      );

      if (!Object.keys(JSON.parse(this.JSONItems)).length) {
        this.appendRow();
      }
    }

    this.JSONError = null;
  };

  liveTableItemsForJSON() {
    // Masked input can buffer value updates; read current controls at conversion time.
    if (typeof document === 'undefined') {
      return this.keyValues;
    }

    const rows = Array.from(document.querySelectorAll('.key-value'));
    if (!rows.length) {
      return this.keyValues;
    }

    const pickControlValue = (row, selectors, { excludeWithin } = {}) => {
      const controls = selectors.flatMap((selector) =>
        Array.from(row.querySelectorAll(selector)),
      );

      const candidates = controls
        .filter((control) => control && typeof control.value === 'string')
        .filter((control) => control.type !== 'hidden')
        .filter((control) => control.getAttribute('aria-hidden') !== 'true')
        .filter((control) => {
          if (!excludeWithin) {
            return true;
          }

          return !excludeWithin.contains(control);
        });

      const visibleCandidates = candidates.filter(
        (control) =>
          control.offsetParent !== null || control.getClientRects().length > 0,
      );

      const preferred = visibleCandidates.length
        ? visibleCandidates
        : candidates;

      if (!preferred.length) {
        return undefined;
      }

      // Prefer controls that currently hold text (avoids empty mirror inputs).
      const withValue = preferred.find((control) => control.value.length > 0);
      return (withValue || preferred[0]).value;
    };

    return rows.map((row, index) => {
      const trackedItem = this.keyValues[index] || { key: '', value: '' };

      const keyValueFromControl = pickControlValue(row, [
        'input[data-test-var-key]',
        'textarea[data-test-var-key]',
        '[data-test-var-key] input',
        '[data-test-var-key] textarea',
      ]);
      const keyWrapper = row.querySelector('[data-test-var-key]');

      const valueFromControl =
        pickControlValue(row, [
          'textarea[data-test-var-value]',
          '[data-test-var-value] textarea',
          'input[data-test-var-value]',
          '[data-test-var-value] input',
        ]) ??
        // Fallback for masked inputs that may not expose stable data-test hooks
        // on the active internal control.
        pickControlValue(row, ['textarea', 'input:not([type="hidden"])'], {
          excludeWithin: keyWrapper,
        });
      const valueWrapper = row.querySelector('[data-test-var-value]');

      const keyValue =
        keyValueFromControl ??
        (typeof keyWrapper?.value === 'string'
          ? keyWrapper.value
          : trackedItem.key);
      const fieldValue =
        valueFromControl ??
        (typeof valueWrapper?.value === 'string'
          ? valueWrapper.value
          : trackedItem.value);

      return {
        key: keyValue,
        value: fieldValue,
      };
    });
  }

  updateCode = (value) => {
    try {
      const parsedValue = JSON.parse(value);
      const hasFormatErrors =
        parsedValue instanceof Array || typeof parsedValue !== 'object';
      if (hasFormatErrors) {
        throw new Error('A Variable must be formatted as a single JSON object');
      }

      this.JSONError = null;
      this.JSONItems = value;
    } catch (error) {
      this.JSONError = error;
    }
  };

  get shouldShowLinkedEntities() {
    return (
      this.args.model.pathLinkedEntities?.job ||
      this.args.model.pathLinkedEntities?.group ||
      this.args.model.pathLinkedEntities?.task ||
      trimPath([this.path]) === 'nomad/jobs'
    );
  }

  get hasUserModifiedAttributes() {
    const compactedBasicKVs = this.keyValues
      .map((kv) => ({ key: kv.key, value: kv.value }))
      .filter((kv) => kv.key || kv.value);
    const compactedPassedKVs = this.args.model.keyValues.filter(
      (kv) => kv.key || kv.value,
    );
    return (
      !isEqual(compactedBasicKVs, compactedPassedKVs) ||
      !isEqual(this.path, this.args.model.path)
    );
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

  confirmExit = (transition) => {
    if (transition.isAborted || transition.queryParamsOnly) return;

    if (this.hasUserModifiedAttributes) {
      if (
        !confirm(
          'Your variable has unsaved changes. Are you sure you want to leave?',
        )
      ) {
        transition.abort();
      } else {
        this.removeExitHandler();
      }
    }
  };

  willDestroy() {
    super.willDestroy(...arguments);
    this.removeExitHandler();
  }

  <template>
    <span hidden {{didUpdate this.onViewChange @view}}></span>
    <span hidden {{didInsert this.establishKeyValues}}></span>
    <form class="new-variables" autocomplete="off" {{on "submit" this.save}}>
      {{#if @model.isNew}}
        {{#unless this.isJobTemplateVariable}}
          <div class="related-entities related-entities-hint">
            <p>Prefix your path with
              <code>nomad/jobs</code>
              to automatically make your variable accessible to all jobs.
              <br />
              Adding job name, group name, or task name will make your variable
              available to that specific job, group, or task.
              <br />
              Format:
              <code>nomad/jobs/&lt;jobname&gt;</code>,
              <code>nomad/jobs/&lt;jobname&gt;/&lt;groupname&gt;</code>,
              <code
              >nomad/jobs/&lt;jobname&gt;/&lt;groupname&gt;/&lt;taskname&gt;</code></p>
          </div>
        {{/unless}}

        {{#if this.shouldShowLinkedEntities}}
          <VariableFormRelatedEntities
            @new={{true}}
            @job={{@model.pathLinkedEntities.job}}
            @group={{@model.pathLinkedEntities.group}}
            @task={{@model.pathLinkedEntities.task}}
            @namespace={{or this.variableNamespace "default"}}
          />
        {{/if}}
      {{/if}}

      {{#if this.hasConflict}}
        <div class="notification conflict is-danger">
          <h3 class="title is-4">Heads up! Your variable has a conflict.</h3>
          <p>This might be because someone else tried saving in the time since
            you've had it open.</p>
          {{#if this.conflictingVariable.modifyTime}}
            <span
              class="tooltip"
              aria-label={{formatTs this.conflictingVariable.modifyTime}}
            >
              {{momentFromNow this.conflictingVariable.modifyTime}}
            </span>
          {{/if}}
          {{#if this.conflictingVariable.items}}
            <pre><code>{{stringifyObject
                  this.conflictingVariable.items
                  whitespace=2
                }}</code></pre>
          {{else}}
            <p>Your ACL token limits your ability to see further details about
              the conflicting variable.</p>
          {{/if}}
          <div class="options">
            <button
              data-test-refresh-button
              type="button"
              class="button"
              {{on "click" this.refresh}}
            >Refresh your browser</button>
            <button
              data-test-overwrite-button
              type="button"
              class="button is-danger"
              {{on "click" this.saveWithOverwrite}}
            >Overwrite anyway</button>
          </div>
        </div>
      {{/if}}

      <div class={{if this.namespaceOptions "path-namespace"}}>
        <HdsFormTextInputField
          @isRequired={{true}}
          @value={{this.path}}
          placeholder="nomad/jobs/my-job/my-group/my-task"
          @isInvalid={{or
            this.duplicatePathWarning
            (and @model.isNew this.hasInvalidPath)
          }}
          {{on "input" this.setModelPath}}
          disabled={{not @model.isNew}}
          {{autofocus}}
          data-test-path-input
          as |F|
        >
          <F.Label>Path</F.Label>

          {{#if this.duplicatePathWarning}}
            <F.Error data-test-duplicate-variable-error>
              There is already a variable located at
              {{this.path}}
              .
              <br />
              Please choose a different path, or
              <LinkTo
                @route="variables.variable.edit"
                @model={{concat
                  this.duplicatePathWarning.path
                  "@"
                  (or this.variableNamespace "default")
                }}
              >
                edit the existing variable
              </LinkTo>
              .
            </F.Error>
          {{/if}}
          {{#if @model.isNew}}
            {{#if this.hasInvalidPath}}
              <F.Error data-test-invalid-path-error>
                Path must contain only alphanumeric or "-", "_", "~", or "/"
                characters, and be fewer than 128 characters in length.
              </F.Error>
            {{/if}}
          {{/if}}
          {{#if this.isJobTemplateVariable}}
            <F.HelperText data-test-job-template-hint>
              Use this variable to generate job templates with
              <HdsCopySnippet
                @textToCopy={{concat
                  "nomad job init -template="
                  this.jobTemplateName
                }}
              />
            </F.HelperText>
          {{/if}}
        </HdsFormTextInputField>

        <VariableFormNamespaceFilter
          @data={{hash
            disabled=(not @model.isNew)
            selection=this.variableNamespace
            namespaceOptions=this.namespaceOptions
          }}
          @fns={{hash
            onSelect=this.setNamespace
            setNamespaceOptions=this.setNamespaceOptions
          }}
        />
      </div>
      {{#if this.isJobTemplateVariable}}
        <VariableFormJobTemplateEditor
          @keyValues={{this.keyValues}}
          @updateKeyValue={{this.updateKeyValue}}
        />
      {{else}}
        {{#if (eq this.view "json")}}
          <div
            class="editor-wrapper boxed-section-body is-full-bleed
              {{if this.JSONError 'error'}}"
          >
            <div
              data-test-json-editor
              {{codeMirror
                content=this.JSONItems
                onUpdate=this.updateCode
                extraKeys=(hash Cmd-Enter=this.save)
              }}
            ></div>
            {{#if this.JSONError}}
              <p class="help is-danger">
                {{this.JSONError}}
              </p>
            {{/if}}
          </div>
        {{else}}
          {{#each this.keyValues as |entry iter|}}
            <div class="key-value">
              <HdsFormTextInputField
                @value={{entry.key}}
                data-test-var-key
                class="input"
                {{autofocus ignore=(eq iter 0)}}
                {{on "input" (fn this.validateKey entry)}}
                as |F|
              >
                <F.Label>Key</F.Label>
              </HdsFormTextInputField>
              <VariableFormInputGroup
                @entry={{entry}}
                @onInput={{fn this.setEntryValue entry}}
              />
              <HdsButton
                @text="Delete"
                @color="critical"
                class="delete-entry-button"
                disabled={{eq this.keyValues.length 1}}
                {{on "click" (fn this.deleteRow entry)}}
              />
              {{#each-in entry.warnings as |key value|}}
                <span class="key-value-error help is-danger">
                  {{value}}
                </span>
              {{/each-in}}
            </div>
          {{/each}}
        {{/if}}
      {{/if}}

      <footer>
        {{#unless this.isJSONView}}
          {{#unless this.isJobTemplateVariable}}
            <HdsButton
              @text="Add More"
              @color="secondary"
              @size="medium"
              @icon="plus"
              disabled={{not
                (and this.lastKeyValue.key this.lastKeyValue.value)
              }}
              {{on "click" this.appendRow}}
              data-test-add-kv
            />
          {{/unless}}
        {{/unless}}
        <HdsButton
          @text="Save {{pluralize 'Variable' this.keyValues.length}}"
          @color="primary"
          type="submit"
          disabled={{this.shouldDisableSave}}
          data-test-submit-var
        />
      </footer>
    </form>
  </template>
}

function normalizeCollection(value) {
  if (!value) {
    return [];
  }

  if (Array.isArray(value)) {
    return value;
  }

  if (value?.slice) {
    return value.slice();
  }

  if (typeof value[Symbol.iterator] === 'function') {
    return Array.from(value);
  }

  return [];
}
