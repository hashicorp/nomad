import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { task } from 'ember-concurrency';
import classic from 'ember-classic-decorator';

import { noCase } from 'no-case';
import { titleCase } from 'title-case';

import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

@classic
export default class JobDispatch extends Component {
  @service can;
  @service store;
  @service config;
  @service router;

  job = null;
  dispatchError = null;
  paramValues = {};
  payload = null;

  @computed('job.definition.Meta', 'job.definition.ParameterizedJob.{MetaOptional,MetaRequired}')
  get params() {
    // Helper for mapping the params into a useable form
    let mapper = (values, isRequired) =>
      values.map(x => {
        let emptyPlaceholder = '';
        let placeholder =
          this.job.definition.Meta != null ? this.job.definition.Meta[x] : emptyPlaceholder;

        return {
          isRequired: isRequired,
          name: x,
          title: titleCase(noCase(x)),

          // Only show the placeholder on fields that aren't mandatory
          placeholder: isRequired ? emptyPlaceholder : placeholder,
        };
      });

    // Fetch the different types of parameters
    let required = mapper(this.job.definition.ParameterizedJob.MetaRequired || [], true);
    let optional = mapper(this.job.definition.ParameterizedJob.MetaOptional || [], false);

    // Return them, required before optional
    return required.concat(optional);
  }

  @computed('job.definition.ParameterizedJob.Payload')
  get hasPayload() {
    return this.job.definition.ParameterizedJob.Payload != 'forbidden';
  }

  @computed('job.definition.ParameterizedJob.Payload')
  get isPayloadRequired() {
    return this.job.definition.ParameterizedJob.Payload == 'required';
  }

  @action
  updateParamValue(name, input) {
    this.paramValues[name] = input.originalTarget.value;
  }

  @task(function*() {
    // Make sure that we have all of the fields that we need
    let isValid = true;
    let required = this.job.definition.ParameterizedJob.MetaRequired || [];
    required.forEach(required => {
      let input = document.getElementById(required);
      isValid &= input.checkValidity();
    });

    // Short out if we are missing fields
    if (!isValid) yield;

    // Try to create the dispatch
    try {
      const dispatch = yield this.job.rawJob.dispatch(this.paramValues, this.payload);

      // Navigate to the newly created instance
      this.router.transitionTo('jobs.job', dispatch.toJSON().dispatchedJobID);
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not dispatch job';
      this.set('dispatchError', error);
    }
  })
  submit;

  @action
  cancel() {
    this.router.transitionTo('jobs.job');
  }

  reset() {
    this.set('dispatchError', null);
    this.set('paramValues', {});
    this.set('payload', null);
  }

  scrollToError() {
    if (!this.get('config.isTest')) {
      window.scrollTo(0, 0);
    }
  }
}
