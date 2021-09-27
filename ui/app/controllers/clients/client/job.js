import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import classic from 'ember-classic-decorator';

@classic
export default class ClientJobController extends Controller.extend(Sortable, Searchable) {
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
  pageSize = 8;

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @computed()
  get searchProps() {
    return ['shortId', 'name'];
  }

  @computed('model.allocations.@each.job')
  get systemJobs() {
    console.log(
      'sysjob\n\n',
      this.model.allocations
        .mapBy('job')
        .filter(job => {
          console.log('job\n\n', job.get('type'));
          return ['system', 'sysbatch'].includes(job.get('type')); // getter doesn't have access to type
        })
        .uniqBy('id')
        .toArray()
    );
    return this.model.allocations
      .mapBy('job')
      .filter(job => ['system', 'sysbatch'].includes(job.get('type')))
      .uniqBy('id');
  }

  @alias('systemJobs') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedJobs;
}
