// @ts-check
import Controller from '@ember/controller';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import classic from 'ember-classic-decorator';
import { action } from '@ember/object';
@classic
export default class IndexController extends Controller.extend(
  WithNamespaceResetting
) {
  @service system;

  queryParams = [
    {
      currentPage: 'page',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
    'activeTask',
    'statusMode',
  ];

  currentPage = 1;

  @alias('model') job;

  sortProperty = 'name';
  sortDescending = false;
  activeTask = null;

  /**
   * @type {('current'|'historical')}
   */
  statusMode = 'current';

  @action
  setActiveTaskQueryParam(task) {
    if (task) {
      this.set('activeTask', `${task.allocation.id}-${task.name}`);
    } else {
      this.set('activeTask', null);
    }
  }

  /**
   * @param {('current'|'historical')} mode
   */
  @action
  setStatusMode(mode) {
    this.set('statusMode', mode);
  }
}
