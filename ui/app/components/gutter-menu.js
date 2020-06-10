import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { computed } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class GutterMenu extends Component {
  @service system;
  @service router;

  @computed('system.namespaces.@each.name')
  get sortedNamespaces() {
    const namespaces = this.get('system.namespaces').toArray() || [];

    return namespaces.sort((a, b) => {
      const aName = a.get('name');
      const bName = b.get('name');

      // Make sure the default namespace is always first in the list
      if (aName === 'default') {
        return -1;
      }
      if (bName === 'default') {
        return 1;
      }

      if (aName < bName) {
        return -1;
      }
      if (aName > bName) {
        return 1;
      }

      return 0;
    });
  }

  onHamburgerClick() {}

  gotoJobsForNamespace(namespace) {
    if (!namespace || !namespace.get('id')) return;

    // Jobs and CSI Volumes are both namespace-sensitive. Changing namespaces is
    // an intent to reset context, but where to reset to depends on where the namespace
    // is being switched from. Jobs take precedence, but if the namespace is switched from
    // a storage-related page, context should be reset to volumes.
    const destination = this.router.currentRouteName.startsWith('csi.') ? 'csi.volumes' : 'jobs';

    this.router.transitionTo(destination, {
      queryParams: { namespace: namespace.get('id') },
    });
  }
}
