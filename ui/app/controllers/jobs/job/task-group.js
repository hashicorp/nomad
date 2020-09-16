import { inject as service } from '@ember/service';
import { alias, readOnly } from '@ember/object/computed';
import Controller from '@ember/controller';
import { action, computed, get } from '@ember/object';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import classic from 'ember-classic-decorator';

@classic
export default class TaskGroupController extends Controller.extend(
    Sortable,
    Searchable,
    WithNamespaceResetting
  ) {
  @service userSettings;
  @service can;

  queryParams = [
    {
      currentPage: 'page',
    },
    {
      searchTerm: 'search',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
  ];

  currentPage = 1;
  @readOnly('userSettings.pageSize') pageSize;

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @computed
  get searchProps() {
    return ['shortId', 'name'];
  }

  @computed('model.allocations.[]')
  get allocations() {
    return this.get('model.allocations') || [];
  }

  @alias('allocations') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedAllocations;

  @computed('model.scaleState.events.@each.time', function() {
    const events = get(this, 'model.scaleState.events');
    if (events) {
      return events.sortBy('time').reverse();
    }
    return [];
  })
  sortedScaleEvents;

  @computed('sortedScaleEvents.@each.{hasCount}', function() {
    const countEventsCount = this.sortedScaleEvents.filterBy('hasCount').length;
    return countEventsCount > 1 && countEventsCount >= this.sortedScaleEvents.length / 2;
  })
  shouldShowScaleEventTimeline;

  @computed('model.job.runningDeployment')
  get tooltipText() {
    if (this.can.cannot('scale job')) return "You aren't allowed to scale task groups";
    if (this.model.job.runningDeployment) return 'You cannot scale task groups during a deployment';
    return undefined;
  }

  @action
  gotoAllocation(allocation) {
    this.transitionToRoute('allocations.allocation', allocation);
  }

  @action
  scaleTaskGroup(count) {
    return this.model.scale(count);
  }
}
