import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  system: service(),
  router: service(),

  sortedNamespaces: computed('system.namespaces.@each.name', function() {
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
  }),

  onHamburgerClick() {},

  gotoJobsForNamespace(namespace) {
    if (!namespace || !namespace.get('id')) return;

    this.router.transitionTo('jobs', {
      queryParams: { namespace: namespace.get('id') },
    });
  },
});
