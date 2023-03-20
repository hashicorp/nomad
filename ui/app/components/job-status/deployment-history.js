// @ts-check
import Component from '@glimmer/component';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';
import { inject as service } from '@ember/service';

export default class JobStatusDeploymentHistoryComponent extends Component {
  @service store;

  async watchDeploymentHistory() {
    const deployment = await this.args.deployment;
    this.watch.perform(deployment, 10000);
  }

  willDestroy() {
    super.willDestroy(...arguments);
    this.watch.cancelAll();
  }

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

  @watchRelationship('allocations', true) watch;
}
