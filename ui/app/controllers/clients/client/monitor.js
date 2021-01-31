import Controller from '@ember/controller';

export default class ClientMonitorController extends Controller {
  queryParams = ['level'];
  level = 'info';
}
