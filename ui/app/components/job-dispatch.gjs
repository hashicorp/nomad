/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { trackedArray } from '@ember/reactive/collections';
import { task } from 'ember-concurrency';
import codeMirror from 'nomad-ui/modifiers/code-mirror';
import { noCase } from 'change-case';
import { titleCase } from 'title-case';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

class MetaField {
  @tracked value;
  @tracked error;

  name;
  required;
  title;

  constructor(meta) {
    this.name = meta.name;
    this.required = meta.required;
    this.title = meta.title;
    this.value = meta.value;
    this.error = meta.error;
  }

  validate() {
    this.error = '';

    if (this.required && !this.value) {
      this.error = `Missing required meta parameter "${this.name}".`;
    }
  }
}

export default class JobDispatch extends Component {
  @service router;
  @service config;

  @tracked metaFields = [];
  @tracked payload = '';
  @tracked payloadHasError = false;
  errors = trackedArray([]);

  constructor() {
    super(...arguments);

    const mapper = (values, required) =>
      values.map(
        (value) =>
          new MetaField({
            name: value,
            required,
            title: titleCase(noCase(value)),
            value: this.args.job.meta ? this.args.job.meta.get(value) : '',
          }),
      );

    const required = mapper(
      this.args.job.parameterizedDetails.MetaRequired || [],
      true,
    );
    const optional = mapper(
      this.args.job.parameterizedDetails.MetaOptional || [],
      false,
    );

    this.metaFields = required.concat(optional);
  }

  get hasPayload() {
    return this.args.job.parameterizedDetails.Payload !== 'forbidden';
  }

  get payloadRequired() {
    return this.args.job.parameterizedDetails.Payload === 'required';
  }

  updateMetaField = (field, event) => {
    field.value = event.target.value;
  };

  updatePayload = (value) => {
    this.payload = value;
  };

  dispatch = () => {
    this.validateForm();
    if (this.errors.length > 0) {
      this.scrollToError();
      return;
    }

    this.onDispatched.perform();
  };

  cancel = () => {
    this.router.transitionTo('jobs.job');
  };

  onDispatched = task({ drop: true }, async () => {
    try {
      const paramValues = {};
      this.metaFields.forEach((field) => {
        paramValues[field.name] = field.value;
      });
      const dispatch = await this.args.job.dispatch(paramValues, this.payload);

      const namespaceId = this.args.job.belongsTo('namespace').id();
      const jobId = namespaceId
        ? `${dispatch.DispatchedJobID}@${namespaceId}`
        : dispatch.DispatchedJobID;

      this.router.transitionTo('jobs.job', jobId);
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not dispatch job';
      this.errors.push(error);
      this.scrollToError();
    }
  });

  scrollToError() {
    if (!this.config.isTest) {
      window.scrollTo(0, 0);
    }
  }

  resetErrors() {
    this.payloadHasError = false;
    this.errors.splice(0, this.errors.length);
  }

  validateForm() {
    this.resetErrors();

    this.metaFields.forEach((field) => {
      field.validate();
      if (field.error) {
        this.errors.push(field.error);
      }
    });

    if (this.payloadRequired && !this.payload) {
      this.errors.push('Missing required payload.');
      this.payloadHasError = true;
    }
  }

  <template>
    {{#if this.errors}}
      <div data-test-dispatch-error class="notification is-danger">
        <h3 class="title is-4" data-test-parse-error-title>Dispatch Error</h3>
        <ul>
          {{#each this.errors as |error|}}
            <li>{{error}}</li>
          {{/each}}
        </ul>
      </div>
    {{/if}}

    <form action="#" onsubmit="return false">
      <h1 class="title">Dispatch an instance of '{{@job.name}}'</h1>

      {{#each this.metaFields as |meta|}}
        <div class="columns">
          <div class="column is-6">
            <div
              data-test-meta-field="{{if meta.required 'required' 'optional'}}"
              class="field"
            >
              <label
                data-test-meta-field-label
                class="label {{if meta.error 'has-text-danger'}}"
                for="{{meta.name}}"
              >
                {{meta.title}}
                {{#if meta.required}}*{{/if}}
              </label>
              <div class="control">
                <input
                  data-test-meta-field-input
                  id="{{meta.name}}"
                  class="input {{if meta.error 'is-danger'}}"
                  type="text"
                  value={{meta.value}}
                  {{on "input" (fn this.updateMetaField meta)}}
                  required={{meta.required}}
                />

                <p class="help {{if meta.error 'has-text-danger'}}">
                  {{#if meta.required}}Required{{else}}Optional{{/if}}
                  Meta Param
                  <span class="badge is-light is-faded">
                    <code>{{meta.name}}</code>
                  </span>
                </p>
              </div>
            </div>
          </div>
        </div>
      {{/each}}

      <div class="boxed-section {{if this.payloadHasError 'is-danger'}}">
        <div data-test-payload-head class="boxed-section-head">
          Payload
          {{#if this.payloadRequired}}*{{/if}}
        </div>
        {{#if this.hasPayload}}
          <div class="boxed-section-body is-full-bleed">
            <div
              data-test-payload-editor
              {{codeMirror
                theme="hashi"
                onUpdate=this.updatePayload
                mode="javascript"
                screenReaderLabel="Payload definition"
              }}
            />
          </div>
        {{else}}
          <div class="boxed-section-body">
            <div data-test-empty-payload-message class="empty-message">
              <h3 class="empty-message-headline">Payload Disabled</h3>
              <p class="empty-message-body">Payload is disabled for this job.</p>
            </div>
          </div>
        {{/if}}
      </div>

      <div>
        <button
          data-test-dispatch-button
          class="button is-primary"
          type="button"
          {{on "click" this.dispatch}}
        >Dispatch</button>
        <button
          data-test-cancel-button
          class="button is-white"
          type="button"
          {{on "click" this.cancel}}
        >Cancel</button>
      </div>
    </form>
  </template>
}
