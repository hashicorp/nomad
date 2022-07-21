import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import classic from 'ember-classic-decorator';

@classic
export default class ServicesRoute extends Route.extend(WithForbiddenState) {
  @service store;
  @service system;

  async model() {
    return RSVP.hash({
      services: this.store.findAll('service'),
    }).catch(notifyForbidden(this));
  }
}
