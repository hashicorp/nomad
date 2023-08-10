/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { A } from '@ember/array';
import { task } from 'ember-concurrency';
import { noCase } from 'no-case';
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

  errors = A([]);

  constructor() {
    super(...arguments);

    // Helper for mapping the params into a useable form.
    const mapper = (values, required) =>
      values.map(
        (x) =>
          new MetaField({
            name: x,
            required,
            title: titleCase(noCase(x)),
            value: this.args.job.meta ? this.args.job.meta.get(x) : '',
          })
      );

    // Fetch the different types of parameters.
    const required = mapper(
      this.args.job.parameterizedDetails.MetaRequired || [],
      true
    );
    const optional = mapper(
      this.args.job.parameterizedDetails.MetaOptional || [],
      false
    );

    // Merge them, required before optional.
    this.metaFields = required.concat(optional);
  }

  get hasPayload() {
    return this.args.job.parameterizedDetails.Payload !== 'forbidden';
  }

  get payloadRequired() {
    return this.args.job.parameterizedDetails.Payload === 'required';
  }

  @action
  dispatch() {
    this.validateForm();
    if (this.errors.length > 0) {
      this.scrollToError();
      return;
    }

    this.onDispatched.perform();
  }

  @action
  cancel() {
    this.router.transitionTo('jobs.job');
  }

  @task({ drop: true }) *onDispatched() {
    // Try to create the dispatch.
    try {
      let paramValues = {};
      this.metaFields.forEach((m) => (paramValues[m.name] = m.value));
      const dispatch = yield this.args.job.dispatch(paramValues, this.payload);

      // Navigate to the newly created instance.
      const namespaceId = this.args.job.belongsTo('namespace').id();
      const jobId = namespaceId
        ? `${dispatch.DispatchedJobID}@${namespaceId}`
        : dispatch.DispatchedJobID;

      this.router.transitionTo('jobs.job', jobId);
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not dispatch job';
      this.errors.pushObject(error);
      this.scrollToError();
    }
  }

  scrollToError() {
    if (!this.config.isTest) {
      window.scrollTo(0, 0);
    }
  }

  resetErrors() {
    this.payloadHasError = false;
    this.errors.clear();
  }

  validateForm() {
    this.resetErrors();

    // Make sure that we have all of the meta fields that we need.
    this.metaFields.forEach((f) => {
      f.validate();
      if (f.error) {
        this.errors.pushObject(f.error);
      }
    });

    // Validate payload.
    if (this.payloadRequired && !this.payload) {
      this.errors.pushObject('Missing required payload.');
      this.payloadHasError = true;
    }
  }
}
