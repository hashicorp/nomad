import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';
import { attributeBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@attributeBindings('data-test-service-status-bar')
export default class ServiceStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  services = null;
  name = null;

  'data-test-service-status-bar' = true;

  @computed('services.{}', 'name')
  get data() {
    const service = this.services && this.services.get(this.name);

    if (!service) {
      return [];
    }

    const pending = service.pending || 0;
    const failing = service.failure || 0;
    const success = service.success || 0;

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
