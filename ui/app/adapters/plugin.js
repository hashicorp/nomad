import Watchable from './watchable';
import classic from 'ember-classic-decorator';

@classic
export default class PluginAdapter extends Watchable {
  queryParamsToAttrs = {
    type: 'type',
  };
}
