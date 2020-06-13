import Controller from '@ember/controller';
import classic from 'ember-classic-decorator';

@classic
export default class ClientMonitorController extends Controller {
  queryParams = [{ level: 'level' }];

  level = 'info';
}
