import AbstractJobPage from './abstract';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';
import messageForError from 'nomad-ui/utils/message-from-adapter-error';

@classic
export default class Periodic extends AbstractJobPage {
  @service store;

  errorMessage = null;

  @action
  forceLaunch() {
    this.job.forcePeriodic().catch((err) => {
      this.set('errorMessage', {
        title: 'Could Not Force Launch',
        description: messageForError(err, 'submit jobs'),
      });
    });
  }

  @action
  clearErrorMessage() {
    this.set('errorMessage', null);
  }
}
