// @ts-check
import Component from '@glimmer/component';

export default class JobStatusPanelComponent extends Component {
  get isActivelyDeploying() {
    return this.args.job.get('latestDeployment.isRunning');
  }
}
