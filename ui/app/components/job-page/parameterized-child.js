import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import PeriodicChildJobPage from './periodic-child';

export default PeriodicChildJobPage.extend({
  payload: alias('job.decodedPayload'),
  payloadJSON: computed('payload', function() {
    let json;
    try {
      json = JSON.parse(this.get('payload'));
    } catch (e) {
      // Swallow error and fall back to plain text rendering
    }
    return json;
  }),
});
