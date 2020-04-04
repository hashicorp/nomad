import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias, readOnly } from '@ember/object/computed';
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
    userSettings: service(),
    volumesController: controller('csi/volumes'),

    isForbidden: alias('volumesController.isForbidden'),

    queryParams: {
      currentPage: 'page',
      sortProperty: 'sort',
      sortDescending: 'desc',
    },

    currentPage: 1,
    pageSize: readOnly('userSettings.pageSize'),

    sortProperty: 'id',
    sortDescending: false,

    /**
      Visible volumes are those that match the selected namespace
    */
    visibleVolumes: computed('model.[]', 'model.@each.parent', function() {
      if (!this.model) return [];

      // Namespace related properties are ommitted from the dependent keys
      // due to a prop invalidation bug caused by region switching.
      const hasNamespaces = this.get('system.namespaces.length');
      const activeNamespace = this.get('system.activeNamespace.id') || 'default';

      return this.model
        .compact()
        .filter(volume => !hasNamespaces || volume.get('namespace.id') === activeNamespace);
    }),

    listToSort: alias('visibleVolumes'),
    sortedVolumes: alias('listSorted'),

    // TODO: Remove once this page gets search capability
    resetPagination() {
      if (this.currentPage != null) {
        this.set('currentPage', 1);
      }
    },

    actions: {
      gotoVolume(volume) {
        this.transitionToRoute('csi.volumes.volume', volume.get('plainId'));
      },
    },
  }
);
