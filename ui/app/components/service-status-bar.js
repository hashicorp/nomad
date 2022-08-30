import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';
import { attributeBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@attributeBindings('data-test-service-status-bar')
export default class ServiceStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  services = null;

  'data-test-service-status-bar' = true;

  @computed('services.@each.status')
  get data() {
    if (!this.services) {
      return [];
    }

    const pending = this.services.filterBy('status', 'pending').length;
    const failing = this.services.filterBy('status', 'failing').length;
    const success = this.services.filterBy('status', 'success').length;

    const [grey, red, green] = ['queued', 'failed', 'complete'];

    return [
      {
        label: 'Pending',
        value: pending,
        className: grey,
      },
      {
        label: 'Failing',
        value: failing,
        className: red,
      },
      {
        label: 'Success',
        value: success,
        className: green,
      },
    ];
  }
}
