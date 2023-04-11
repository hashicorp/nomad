// @ts-check
import Component from '@ember/component';
import { task } from 'ember-concurrency';
import { inject as service } from '@ember/service';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class Title extends Component {
  @service router;

  job = null;
  title = null;

  handleError() {}

  /**
   * @param {boolean} withNNotifications - Whether to show a toast on success, as when triggered by keyboard shortcut
   */
  @task(function* (withNNotifications = false) {
    try {
      const job = this.job;
      yield job.stop();
      // Eagerly update the job status to avoid flickering
      this.job.set('status', 'dead');
      if (withNNotifications) {
        this.notifications.add({
          title: 'Job Stopped',
          message: `${this.job.name} has been stopped`,
          color: 'success',
        });
      }
    } catch (err) {
      this.handleError({
        title: 'Could Not Stop Job',
        description: messageFromAdapterError(err, 'stop jobs'),
      });
    }
  })
  stopJob;

  @task(function* () {
    try {
      const job = this.job;
      yield job.purge();
      this.flashMessages.add({
        title: 'Job Purged',
        message: `You have purged ${this.job.name}`,
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });
      this.router.transitionTo('jobs');
    } catch (err) {
      this.handleError({
        title: 'Error purging job',
        description: messageFromAdapterError(err, 'purge jobs'),
      });
    }
  })
  purgeJob;

  /**
   * @param {boolean} withNotifications - Whether to show a toast on success, as when triggered by keyboard shortcut
   */
  @task(function* (withNotifications = false) {
    const job = this.job;
    const definition = yield job.fetchRawDefinition();

    delete definition.Stop;
    job.set('_newDefinition', JSON.stringify(definition));

    try {
      yield job.parse();
      yield job.update();
      // Eagerly update the job status to avoid flickering
      job.set('status', 'running');
      if (withNotifications) {
        this.notifications.add({
          title: 'Job Started',
          message: `${this.job.name} has started`,
          color: 'success',
        });
      }
    } catch (err) {
      this.handleError({
        title: 'Could Not Start Job',
        description: messageFromAdapterError(err, 'start jobs'),
      });
    }
  })
  startJob;
}
