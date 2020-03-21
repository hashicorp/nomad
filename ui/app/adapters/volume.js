import Watchable from './watchable';
import WithNamespaceIDs from 'nomad-ui/mixins/with-namespace-ids';

export default Watchable.extend(WithNamespaceIDs, {
  queryParamsToAttrs: {
    type: 'type',
    plugin_id: 'plugin.id',
  },
});
