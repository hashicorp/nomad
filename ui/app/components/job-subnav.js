import { inject as service } from '@ember/service';
import Component from '@glimmer/component';

export default class JobSubnav extends Component {
  @service can;

  get shouldRenderClientsTab() {
    const { job } = this.args;
    return (
      job?.hasClientStatus && !job?.hasChildren && this.can.can('read client')
    );
  }
}
