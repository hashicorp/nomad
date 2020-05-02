import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default Route.extend(WithForbiddenState, {
  store: service(),

  breadcrumbs: [
    {
      label: 'Storage',
      args: ['csi.index'],
    },
  ],

  model() {
    return this.store.query('plugin', { type: 'csi' }).catch(notifyForbidden(this));
  },
});
