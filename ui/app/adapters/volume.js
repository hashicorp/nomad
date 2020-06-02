import Watchable from './watchable';
import WithNamespaceIDs from 'nomad-ui/mixins/with-namespace-ids';

export default Watchable.extend(WithNamespaceIDs, {
  queryParamsToAttrs: Object.freeze({
    type: 'type',
    plugin_id: 'plugin.id',
  }),
});
