import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';

export default Controller.extend(
  SortableFactory([
    'id',
    'schedulable',
    'controllersHealthyProportion',
    'nodesHealthyProportion',
    'provider',
  ]),
  {
    system: service(),
    csiController: controller('csi'),

    isForbidden: alias('csiController.isForbidden'),

    queryParams: {
      currentPage: 'page',
      sortProperty: 'sort',
      sortDescending: 'desc',
    },

    currentPage: 1,
    pageSize: 10,

    sortProperty: 'id',
    sortDescending: true,

    listToSort: alias('model'),
    sortedVolumes: alias('listSorted'),
  }
);
