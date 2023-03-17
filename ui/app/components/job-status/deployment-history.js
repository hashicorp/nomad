// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class JobStatusDeploymentHistoryComponent extends Component {
  get history() {
    return this.args.deployment
      .get('allocations')
      .map((a) =>
        a
          .get('states')
          .map((s) => s.events.content)
          .flat()
      )
      .flat()
      .sort((a, b) => a.get('time') - b.get('time'))
      .reverse();
  }

  @action fetchDeploymentHistory() {
    this.args.deployment.get('allocations');
  }
}
