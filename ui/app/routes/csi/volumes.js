import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default Route.extend(WithForbiddenState, {
  breadcrumbs: [
    {
      label: 'CSI',
      args: ['csi.volumes.index'],
    },
  ],

  model() {
    // return this.store.findAll('volume', { reload: true }).catch(notifyForbidden(this));
    return this.store
      .query('volume', { type: 'csi' })
      .then(volumes => {
        volumes.forEach(volume => volume.plugin);
        return volumes;
      })
      .catch(notifyForbidden(this));
  },
});
