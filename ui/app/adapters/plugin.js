import Watchable from './watchable';

export default class Plugin extends Watchable {
  queryParamsToAttrs = {
    type: 'type',
  };
}
