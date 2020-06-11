import Watchable from './watchable';

export default class PluginAdapter extends Watchable {
  queryParamsToAttrs = Object.freeze({
    type: 'type',
  });
}
