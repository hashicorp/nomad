import WatchableNamespaceIDs from './watchable-namespace-ids';

export default WatchableNamespaceIDs.extend({
  queryParamsToAttrs: Object.freeze({
    type: 'type',
    plugin_id: 'plugin.id',
  }),
});
