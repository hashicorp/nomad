import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import PeriodicChildJobPage from './periodic-child';
import classic from 'ember-classic-decorator';
import jobClientStatus from 'nomad-ui/utils/properties/job-client-status';

@classic
export default class ParameterizedChild extends PeriodicChildJobPage {
  @alias('job.decodedPayload') payload;
  @service store;

  @computed('payload')
  get payloadJSON() {
    let json;
    try {
      json = JSON.parse(this.payload);
    } catch (e) {
      // Swallow error and fall back to plain text rendering
    }
    return json;
  }

  @jobClientStatus('nodes', 'job') jobClientStatus;

  get nodes() {
    return this.store.peekAll('node');
  }
}
