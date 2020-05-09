import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias, readOnly } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import Searchable from 'nomad-ui/mixins/searchable';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';

export default Controller.extend(
  SortableFactory([
    'plainId',
    'controllersHealthyProportion',
    'nodesHealthyProportion',
    'provider',
  ]),
  Searchable,
  {
    userSettings: service(),
    pluginsController: controller('csi/plugins'),

    isForbidden: alias('pluginsController.isForbidden'),

    queryParams: {
      currentPage: 'page',
      searchTerm: 'search',
      sortProperty: 'sort',
      sortDescending: 'desc',
    },

    currentPage: 1,
    pageSize: readOnly('userSettings.pageSize'),

    searchProps: computed(() => ['id']),
    fuzzySearchProps: computed(() => ['id']),

    sortProperty: 'id',
    sortDescending: false,

    listToSort: alias('model'),
    listToSearch: alias('listSorted'),
    sortedPlugins: alias('listSearched'),

    actions: {
      gotoPlugin(plugin, event) {
        lazyClick([() => this.transitionToRoute('csi.plugins.plugin', plugin.plainId), event]);
      },
    },
  }
);
