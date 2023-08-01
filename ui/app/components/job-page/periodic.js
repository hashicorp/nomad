import { action } from '@ember/object';
import Component from '@glimmer/component';

export default class Periodic extends Component {
  @action
  forceLaunch(setError) {
    this.args.job.forcePeriodic().catch((err) => {
      setError(err);
    });
  }
}
