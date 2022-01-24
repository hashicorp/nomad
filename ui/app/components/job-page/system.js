import AbstractJobPage from './abstract';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';
import jobClientStatus from 'nomad-ui/utils/properties/job-client-status';

@classic
export default class System extends AbstractJobPage {
  @service store;

  @jobClientStatus('nodes', 'job') jobClientStatus;

  get nodes() {
    return this.store.peekAll('node');
  }
}
