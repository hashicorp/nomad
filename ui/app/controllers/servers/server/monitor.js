import Controller from '@ember/controller';
import classic from 'ember-classic-decorator';

@classic
export default class ServerMonitorController extends Controller {
  queryParams = [{ level: 'level' }];

  level = 'info';
}
