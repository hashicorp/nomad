import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { alias, readOnly } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import Searchable from 'nomad-ui/mixins/searchable';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';
import classic from 'ember-classic-decorator';

@classic
export default class IndexController extends Controller.extend(
    SortableFactory([
      'id',
      'schedulable',
      'controllersHealthyProportion',
      'nodesHealthyProportion',
      'provider',
    ]),
    Searchable
  ) {
  @service system;
  @service userSettings;
  @controller('csi/volumes') volumesController;

  @alias('volumesController.isForbidden')
  isForbidden;

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

  sortProperty = 'id';
  sortDescending = false;

  @computed
  get searchProps() {
    return ['name'];
  }

  @computed
  get fuzzySearchProps() {
    return ['name'];
  }

  fuzzySearchEnabled = true;

  /**
    Visible volumes are those that match the selected namespace
  */
  @computed('model.{[],@each.parent}')
  get visibleVolumes() {
    if (!this.model) return [];

    // Namespace related properties are ommitted from the dependent keys
    // due to a prop invalidation bug caused by region switching.
    const hasNamespaces = this.get('system.namespaces.length');
    const activeNamespace = this.get('system.activeNamespace.id') || 'default';

    return this.model
      .compact()
      .filter(volume => !hasNamespaces || volume.get('namespace.id') === activeNamespace);
  }

  @alias('visibleVolumes') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedVolumes;

  @action
  gotoVolume(volume, event) {
    lazyClick([() => this.transitionToRoute('csi.volumes.volume', volume.get('plainId')), event]);
  }
}
