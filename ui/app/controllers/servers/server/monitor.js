import Controller from '@ember/controller';

export default class ServerMonitorController extends Controller {
  queryParams = ['level'];
  level = 'info';
}
