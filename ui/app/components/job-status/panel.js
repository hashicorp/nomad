// @ts-check
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class JobStatusPanelComponent extends Component {
  @service store;

  get isActivelyDeploying() {
    return this.args.job.get('latestDeployment.isRunning');
  }

  get nodes() {
    return this.store.peekAll('node');
  }
}
