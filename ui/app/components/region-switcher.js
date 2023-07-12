import Component from '@ember/component';
import { computed } from '@ember/object';
import { inject as service } from '@ember/service';
import classic from 'ember-classic-decorator';

@classic
export default class RegionSwitcher extends Component {
  @service system;
  @service router;
  @service store;

  @computed('system.regions')
  get sortedRegions() {
    return this.get('system.regions').toArray().sort();
  }

  gotoRegion(region) {
    this.router.transitionTo('jobs', {
      queryParams: { region },
    });
  }
}
