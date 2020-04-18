import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default Route.extend(WithForbiddenState, {
  system: service(),
  store: service(),

  breadcrumbs: [
    {
      label: 'Storage',
      args: ['csi.index'],
    },
  ],

  queryParams: {
    volumeNamespace: {
      refreshModel: true,
    },
  },

  beforeModel(transition) {
    return this.get('system.namespaces').then(namespaces => {
      const queryParam = transition.to.queryParams.namespace;
      this.set('system.activeNamespace', queryParam || 'default');

      return namespaces;
    });
  },

  model() {
    return this.store
      .query('volume', { type: 'csi' })
      .then(volumes => {
        volumes.forEach(volume => volume.plugin);
        return volumes;
      })
      .catch(notifyForbidden(this));
  },
});
