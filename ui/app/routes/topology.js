import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import classic from 'ember-classic-decorator';
import RSVP from 'rsvp';

@classic
export default class TopologyRoute extends Route.extend(WithForbiddenState) {
  @service store;
  @service system;

  breadcrumbs = [
    {
      label: 'Topology',
      args: ['topology'],
    },
  ];

  model() {
    return RSVP.hash({
      allocations: this.store.findAll('allocation'),
      jobs: this.store.findAll('job'),
      nodes: this.store.findAll('node'),
    }).catch(notifyForbidden(this));
  }
}
