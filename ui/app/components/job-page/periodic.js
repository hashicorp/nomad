import AbstractJobPage from './abstract';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class Periodic extends AbstractJobPage {
  @service store;

  errorMessage = null;

  @action
  forceLaunch() {
    this.job.forcePeriodic().catch(() => {
      this.set('errorMessage', {
        title: 'Could Not Force Launch',
        description: 'Your ACL token does not grant permission to submit jobs.',
      });
    });
  }

  @action
  clearErrorMessage() {
    this.set('errorMessage', null);
  }
}
