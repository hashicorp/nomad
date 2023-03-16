// @ts-check
import Component from '@glimmer/component';

export default class JobStatusDeploymentHistoryComponent extends Component {
  get history() {
    console.log('well lets start from the top, what can I know about the deplo allocs', this.args.deployment);
    console.log('lets gooooooo', this.args.deployment.get('allocations').map(a => a.get('states').map((s) => s.events.content).flat()).flat())

    return this.args.deployment.get('allocations').map(a => a.get('states').map((s) => s.events.content).flat()).flat();
  }
}
